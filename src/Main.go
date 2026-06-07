package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ---------------------------------------------------------------------------
// CLI flags
// ---------------------------------------------------------------------------

const Version = "v1.0.0"

var (
	flagASTOnly = flag.Bool("ast-only", false, "Parse and print AST; skip codegen and GCC")
	flagEmitC   = flag.Bool("emit-c", false, "Write generated_output.c but skip GCC")
	flagOutput  = flag.String("output", "MerlinKernel", "Output binary name")
	flagVerbose = flag.Bool("verbose", false, "Print pipeline stage progress to stderr")
	flagLink    = flag.String("link", "", "Link against a library (e.g. -link math)")
	flagOpt     = flag.String("opt", "O", "Optimization level (O, O1, O2, O3)")
	flagVersion = flag.Bool("version", false, "Print version and exit")
)

// ---------------------------------------------------------------------------
// Entry point
// ---------------------------------------------------------------------------

func main() {
	flag.Usage = usage
	flag.Parse()

	if *flagVersion {
		fmt.Printf("Merlin %s\n", Version)
		os.Exit(0)
	}

	args := flag.Args()
	if len(args) < 1 {
		usage()
		os.Exit(1)
	}

	srcPath := args[0]
	if !strings.HasSuffix(srcPath, ".mrl") {
		fatal("source file must have .mrl extension: %s", srcPath)
	}

	// Set default output name to the filename without extension
	if *flagOutput == "MerlinKernel" {
		base := srcPath
		if lastSlash := strings.LastIndex(base, "/"); lastSlash != -1 {
			base = base[lastSlash+1:]
		}
		*flagOutput = strings.TrimSuffix(base, ".mrl")
	}

	// Determine generated C filename (e.g. test.mrl -> test.c)
	cFileName := srcPath
	if lastSlash := strings.LastIndex(cFileName, "/"); lastSlash != -1 {
		cFileName = cFileName[lastSlash+1:]
	}
	cFileName = strings.TrimSuffix(cFileName, ".mrl") + ".c"

	// --- Stage 1: Read source ---
	logVerbose("reading source file: %s", srcPath)
	src, err := os.ReadFile(srcPath)
	if err != nil {
		fatal("cannot read source file: %v", err)
	}

	// --- Stage 2: Lex ---
	logVerbose("lexing...")
	lexer := NewLexerWithFile(string(src), srcPath)
	tokens := lexer.Tokenize()
	logVerbose("lexed %d tokens", len(tokens))

	// --- Stage 3: Parse ---
	logVerbose("parsing...")
	parser := NewParserWithFile(tokens, string(src), srcPath)
	program := parser.ParseProgram()
	logVerbose("parsed %d top-level nodes (%d imports)",
		len(program.Statements), len(program.Imports))

	// Check for parse errors
	if len(parser.errors) > 0 {
		for _, err := range parser.errors {
			fmt.Fprintln(os.Stderr, err)
		}
		os.Exit(1)
	}

	// --- Stage 4: Print AST (--ast-only) ---
	if *flagASTOnly {
		printAST(program)
		os.Exit(0)
	}

	// --- Stage 5: Semantic analysis (eval.go — OpenCode) ---
	logVerbose("semantic analysis...")
	checker := NewTypeChecker()
	checker.Check(program, srcPath, true)

	// Check for type-check errors
	if len(checker.errors) > 0 {
		for _, err := range checker.errors {
			fmt.Fprintln(os.Stderr, err)
		}
		os.Exit(1)
	}

	// --- Stage 6: Code generation (codegen.go — OpenCode) ---
	logVerbose("generating C code...")
	gen := NewCodeGen()
	gen.SetConcreteDecls(checker.ConcreteDecls)

	// Register imported modules in codegen for correct CallExpr emission
	for _, imp := range program.Imports {
		modName := imp.Path
		if idx := strings.Index(modName, "/"); idx != -1 { modName = modName[idx+1:] }
		modName = strings.TrimSuffix(modName, ".mrl")
		gen.importedModules[modName] = true
	}
	
	// We need to collect all programs (main + imports) for codegen
	allProgs := make(map[string]*Program)
	moduleName := func(path string) string {
		n := path
		if idx := strings.Index(n, "/"); idx != -1 { n = n[idx+1:] }
		return strings.TrimSuffix(n, ".mrl")
	}
	var collect func(p *Program, path string)
	collect = func(p *Program, path string) {
		allProgs[path] = p
		for _, imp := range p.Imports {
			mName := moduleName(imp.Path)
			searchPaths := []string{
				fmt.Sprintf("%s/%s.mrl", filepath.Dir(path), imp.Path),
				fmt.Sprintf("std/%s.mrl", imp.Path),
				fmt.Sprintf("packages/%s.mrl", imp.Path),
			}
			pkgSearchPaths := []string{
				filepath.Join(filepath.Dir(path), imp.Path),
				filepath.Join("std", imp.Path),
				filepath.Join("packages", imp.Path),
			}
			var foundPath string
			for _, sp := range searchPaths {
				if _, err := os.Stat(sp); err == nil {
					foundPath = sp
					break
				}
			}
			if foundPath == "" {
				for _, dir := range pkgSearchPaths {
					mrlPath := filepath.Join(dir, mName+".mrl")
					if _, err := os.Stat(mrlPath); err == nil {
						foundPath = mrlPath
						break
					}
				}
			}
			if foundPath != "" {
				src, err := os.ReadFile(foundPath)
				if err == nil {
					lx := NewLexerWithFile(string(src), foundPath)
					px := NewParserWithFile(lx.Tokenize(), string(src), foundPath)
					collect(px.ParseProgram(), foundPath)
				}
			}
		}
	}
	collect(program, srcPath)

	// Collect link libraries from external function declarations across all programs
	for _, p := range allProgs {
		for _, stmt := range p.Statements {
			if ext, ok := stmt.(*ExternalFuncDecl); ok && ext.LinkLib != "" {
				checker.requiredLibs[ext.LinkLib] = true
			}
		}
	}

	cSrc := gen.Emit(program, allProgs)
	err = os.WriteFile(cFileName, []byte(cSrc), 0644)
	if err != nil {
		fatal("cannot write %s: %v", cFileName, err)
	}

	if *flagEmitC {
		logVerbose("--emit-c set; skipping GCC")
		os.Exit(0)
	}

	// --- Stage 7: GCC compilation (OpenCode wires this up after codegen) ---
	logVerbose("compiling with GCC...")
	args_gcc := []string{fmt.Sprintf("-%s", *flagOpt), "-fno-strict-aliasing", "-march=native", cFileName, "-o", *flagOutput}
	
	// Automatic library paths (-L)
	for path := range checker.libraryPaths {
		args_gcc = append(args_gcc, fmt.Sprintf("-L%s", path))
	}
	
	// Automatic library names (-l)
	for lib := range checker.requiredLibs {
		args_gcc = append(args_gcc, fmt.Sprintf("-l%s", lib))
	}
	
	if *flagLink != "" {
		args_gcc = append(args_gcc, fmt.Sprintf("-l%s", *flagLink))
	}
	cmd := exec.Command("gcc", args_gcc...)
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fatal("GCC compilation failed")
	}
	fmt.Printf("compiled: %s\n", *flagOutput)
	os.Exit(0)
}

// ---------------------------------------------------------------------------
// Utilities
// ---------------------------------------------------------------------------

func logVerbose(format string, args ...any) {
	if *flagVerbose {
		fmt.Fprintf(os.Stderr, "[merlin] "+format+"\n", args...)
	}
}

func fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "[merlin] FATAL: "+format+"\n", args...)
	os.Exit(1)
}

func usage() {
	fmt.Fprintln(os.Stderr, `Merlin `+Version+`
Usage: merlin <source.mrl> [flags]

Flags:
  --ast-only      Parse and print AST as JSON; skip codegen and GCC
  --emit-c        Emit generated_output.c but skip GCC
  --output <bin>  Output binary name (default: <source>.mrl basename)
  --verbose       Print pipeline stage info to stderr
  --version       Print version and exit
  --link <lib>    Link against a system library (e.g. -link m for libm.so)
  --opt <level>   Optimization level (O, O1, O2, O3) - default: O`)
}
