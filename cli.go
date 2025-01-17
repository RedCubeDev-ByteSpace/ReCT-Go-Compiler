package main

import (
	"bytes"
	"flag"
	"fmt"
	"github.com/ReCT-Lang/ReCT-Go-Compiler/binder"
	"github.com/ReCT-Lang/ReCT-Go-Compiler/emitter"
	"github.com/ReCT-Lang/ReCT-Go-Compiler/evaluator"
	"github.com/ReCT-Lang/ReCT-Go-Compiler/lexer"
	"github.com/ReCT-Lang/ReCT-Go-Compiler/nodes"
	"github.com/ReCT-Lang/ReCT-Go-Compiler/packager"
	"github.com/ReCT-Lang/ReCT-Go-Compiler/parser"
	"github.com/ReCT-Lang/ReCT-Go-Compiler/preprocessor"
	"github.com/ReCT-Lang/ReCT-Go-Compiler/print"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
)

/* cli.go handles flags and command line arguments for the project
 * Everything is documented below, this was moved away from its own package
 * formally cli/cli.go because it makes more sense for the flags to be processed
 * inside the main package - you gain no benefit from having cli as its own package.
 */

// Defaults
// These are the default values of all flags
// The values are set using the flag library, but they are also commented below
var helpFlag bool      // false -h
var interpretFlag bool // false  -i
var showVersion bool   // false -v
var fileLog bool       // false -l
var debug bool         // -xx
var tests bool         // Just for running test file like test.rct ( -t )
var files []string
var lookup int // For looking up error details
var outputPath string
var llvm bool
var optimize bool
var packageIncludePath string

var CompileAsPackage bool
var PackageName string

// Constants that are used throughout code
// Should be updated when necessary
const executableName string = "rgoc"                         // in case we change it later
const discordInvite string = "https://discord.gg/kk9MsnABdF" // infinite link
const currentVersion string = "1.1"

// Init initializes and processes (parses) compiler flags
func Init() {
	flag.BoolVar(&helpFlag, "h", false, "Shows this help message")
	flag.BoolVar(&interpretFlag, "i", false, "Enables interpreter mode, source code will be interpreted instead of compiled.")
	flag.BoolVar(&showVersion, "v", false, "Shows current ReCT version the compiler supports")
	flag.BoolVar(&fileLog, "l", false, "Logs process information in a log file")
	flag.BoolVar(&debug, "xx", false, "Shows brief process information in the command line")
	// Test (-t) will not be in the help message as it's only really going ot be used for testing compiler features.
	flag.BoolVar(&tests, "t", false, "For compiler test files (developers only)")
	flag.IntVar(&lookup, "lookup", 0, "Displays further detail and examples of Errors")
	flag.StringVar(&outputPath, "o", "", "Output file")
	flag.BoolVar(&llvm, "llvm", false, "Compile to LLVM Module")
	flag.StringVar(&PackageName, "package", "", "Compile as a package with the given name")
	flag.BoolVar(&optimize, "O", false, "Use compiler optimizations")
	flag.StringVar(&packageIncludePath, "pi", "", "Custom package include path")
	flag.Parse()

	// needs to be called after flag.Parse() or it'll be empty lol
	files = flag.Args() // Other arguments like executable name or files
}

// ProcessFlags goes through each flag and decides how they have an effect on the output of the compiler
func ProcessFlags() {
	// Mmm test has the highest priority
	if tests {
		RunTests()

	} else if showVersion { // Show version has higher priority than help menu
		Version()

	} else if helpFlag {
		// If they use "-h" or only enter the executable name "rgoc"
		// Show the help menu because they're obviously insane.
		Help()

	} else if lookup != 0 { // 0 = No look up (default value)
		// If you user requests error code look up
		print.LookUp(print.ErrorCode(lookup))

	} else {

		// this is handled here now
		if len(files) <= 0 {
			Help()
			return

		} else if interpretFlag {
			InterpretFile(files[0])

		} else {
			if PackageName != "" {
				emitter.CompileAsPackage = true
				emitter.PackageName = PackageName
			}

			// get the rgoc executable path
			ex, err := os.Executable()
			if err != nil {
				panic(err)
			}
			exPath := filepath.Dir(ex)

			// append the executable path as a valid package location
			packager.PackagePaths = append(packager.PackagePaths, exPath+"/packages") // standard package dir

			if packageIncludePath != "" {
				packager.PackagePaths = append(packager.PackagePaths, packageIncludePath)
			}

			CompileFiles(files)
		}
	}
}

// InterpretFile runs everything to interpret the files, currently only supports up to one file
func InterpretFile(file string) {
	boundProgram := Prepare(file)
	//print.PrintC(print.Cyan, "-> Evaluating!")
	evaluator.Evaluate(boundProgram)
}

// CompileFiles compiles everything and outputs an LLVM file
func CompileFiles(files []string) {
	// remember the cwd
	cwd, _ := os.Getwd()

	// get the rgoc executable path
	ex, err := os.Executable()
	if err != nil {
		panic(err)
	}
	exPath := filepath.Dir(ex)

	// lex, parse, and bind the program
	boundProgram, args := PrepareMultifile(files)

	if debug {
		emitter.VerboseARC = true
	}

	emitter.CurrentVersion = currentVersion
	emitter.SourceFile = files[0]

	module := emitter.Emit(boundProgram, true)
	// any errors after emitting? -> die();
	print.CrashIfErrorsFound()

	//fmt.Println(module)
	output := module.String()

	print.PrintC(print.Green, "Compiled module successfully!")

	// if we're just after the LL Module
	if llvm {
		// check if we need to generate a path
		outPath := outputPath
		if outputPath == "" {
			ext := path.Ext(files[0])
			outPath = files[0][0:len(files[0])-len(ext)] + ".ll"
		}

		// change the cwd back
		os.Chdir(cwd)

		// write the module
		os.WriteFile(outPath, []byte(output), 0644)

		// we're done
		return
	}

	// if thats not the case -> spin up a temp dir
	os.Mkdir("./.tmp", os.ModePerm)

	// write the module there
	os.WriteFile("./.tmp/prgout.ll", []byte(output), 0644)

	// opt all used packages
	linkFiles := make([]string, 0)
	for _, pck := range packager.PackagesSoFar {
		// run the opt command
		cmd := exec.Command("opt", "./packages/"+pck.Name+".ll", "-o", "./.tmp/"+pck.Name+".bc")
		o, err := cmd.CombinedOutput()

		// if something goes wrong -> report that to the user
		if err != nil {
			print.PrintCF(print.Red, "Error compiling package '%s' into llvm bitcode!", pck.Name)
			fmt.Println(err.Error())
			fmt.Println(string(o))

			// delete the temp dir and die
			os.RemoveAll("./.tmp")
			os.Exit(-1)
		}

		// if everything is fine, add this file to the linking list
		linkFiles = append(linkFiles, "./.tmp/"+pck.Name+".bc")
	}

	// opt this module
	cmd := exec.Command("opt", "./.tmp/prgout.ll", "-o", "./.tmp/prgout.bc")
	o, err := cmd.CombinedOutput()

	// if something goes wrong -> report that to the user
	if err != nil {
		print.PrintC(print.Red, "Error compiling this llvm module into llvm bitcode!")
		fmt.Println(err.Error())
		fmt.Println(string(o))

		// delete the temp dir and die
		os.RemoveAll("./.tmp")
		os.Exit(-1)
	}

	// if everything is fine, add our module to the linking list
	linkFiles = append(linkFiles, "./.tmp/prgout.bc")

	// do we have an adapter module which needs to be included?
	if emitter.AdapterModule != "" {
		// opt the adapter module
		cmd := exec.Command("opt", "-", "-o", "./.tmp/adpout.bc")

		// read code from stdin
		buffer := bytes.Buffer{}
		buffer.Write([]byte(emitter.AdapterModule))
		cmd.Stdin = &buffer

		o, err := cmd.CombinedOutput()

		// if something goes wrong -> report that to the user
		if err != nil {
			print.PrintC(print.Red, "Error compiling adapter module into llvm bitcode!")
			fmt.Println(err.Error())
			fmt.Println(string(o))

			// delete the temp dir and die
			os.RemoveAll("./.tmp")
			os.Exit(-1)
		}

		// if everything is fine, add the adapter module to the linking list
		linkFiles = append(linkFiles, "./.tmp/adpout.bc")
	}

	// add the systemlib to the linklist
	linkFiles = append(linkFiles, "./systemlib/systemlib_lin.bc")

	// args for llvm link
	linkArgs := append(linkFiles, "-o", "./.tmp/completeout.bc")

	// call the llvm linker
	cmd = exec.Command("llvm-link", linkArgs...)
	o, err = cmd.CombinedOutput()

	// if something goes wrong -> report that to the user
	if err != nil {
		print.PrintC(print.Red, "Error linking llvm bitcode!")
		fmt.Println(cmd)
		fmt.Println(err.Error())
		fmt.Println(string(o))

		// delete the temp dir and die
		os.RemoveAll("./.tmp")
		os.Exit(-1)
	}

	// lastly, clang the bitcode into an executable
	outPath := outputPath
	if outputPath == "" {
		ext := path.Ext(files[0])
		outPath = files[0][0 : len(files[0])-len(ext)]
	}
	os.Chdir(cwd)

	// optimize?
	opt := "-O0"
	if optimize {
		opt = "-O3"
	}

	// clang arguments
	clargs := []string{opt, "-lm", "-lgc", "-pthread", "-rdynamic", exPath + "/.tmp/completeout.bc", "-o", outPath}
	clargs = append(clargs, args...)

	// call clang
	cmd = exec.Command("clang", clargs...)
	o, err = cmd.CombinedOutput()

	// if something goes wrong -> report that to the user
	if err != nil {
		print.PrintC(print.Red, "Error compiling llvm bitcode to executable!")
		fmt.Println(err.Error())
		fmt.Println(string(o))

		// delete the temp dir and die
		//os.RemoveAll("./.tmp")
		os.Exit(-1)
	}

	// utterly destroy the temp dir
	os.RemoveAll("./.tmp")

	print.PrintC(print.Cyan, "Compiled executable successfully!")
}

// Prepare runs the lexer, parser, binder, and lowerer. This is used before evaluation or emitting.
func Prepare(file string) binder.BoundProgram {
	if debug {
		print.WriteC(print.Green, "-> Lexing...  ")
	}

	code := lexer.ReadFile(file)
	tokens := lexer.Lex(code, file)
	// any errors after lexing? -> die();
	print.CrashIfErrorsFound()

	if debug {
		print.PrintC(print.Green, "Done!")
	}

	if debug {
		print.WriteC(print.Yellow, "-> Parsing... ")
	}

	members := parser.Parse(tokens)

	// any errors after parsing? -> die();
	print.CrashIfErrorsFound()

	if debug {
		print.PrintC(print.Green, "Done!")
		for _, mem := range members {
			mem.Print("")
		}
	}

	// change the current working directory
	ex, err := os.Executable()
	if err != nil {
		panic(err)
	}
	exPath := filepath.Dir(ex)
	os.Chdir(exPath)

	if debug {
		print.WriteC(print.Red, "-> Binding... ")
	}

	boundProgram := binder.BindProgram(members)

	// any errors after binding? -> die();
	print.CrashIfErrorsFound()

	if debug {
		print.PrintC(print.Green, "Done!")
		boundProgram.Print()
	}

	//boundProgram.Print()
	//boundProgram.PrintStatements()

	return boundProgram
}

// PrepareMultifile runs the lexer and parser for each given file and then feeds the result to the binder and lowerer.
// This is used before evaluation or emitting.
func PrepareMultifile(files []string) (binder.BoundProgram, []string) {
	if debug {
		print.WriteC(print.Green, "-> Preprocessing + Lexing...  ")
	}

	arguments := make([]string, 0)
	lexes := make([][]lexer.Token, 0) // all tokens of all lexer runs

	// lex all given files
	for i := 0; i < len(files); i++ {
		file := files[i]
		code := []rune(preprocessor.Preprocess(file, &files, &arguments))
		// any errors after preprocessing? -> die();
		print.CrashIfErrorsFound()

		//code := lexer.ReadFile(file)
		tokens := lexer.Lex(code, file)
		// any errors after lexing? -> die();
		print.CrashIfErrorsFound()

		lexes = append(lexes, tokens)

		if debug {
			for _, token := range tokens {
				fmt.Println(token.String(true))
			}
		}
	}

	if debug {
		print.PrintC(print.Green, "Done!")
		print.WriteC(print.Yellow, "-> Parsing... ")
	}

	memberList := make([]nodes.MemberNode, 0)

	for _, lex := range lexes {
		members := parser.Parse(lex)

		// we mergin'
		memberList = append(members, memberList...)
	}

	// any errors after parsing? -> die();
	print.CrashIfErrorsFound()

	if debug {
		print.PrintC(print.Green, "Done!")
		for _, mem := range memberList {
			mem.Print("")
		}
	}

	// change the current working directory
	ex, err := os.Executable()
	if err != nil {
		panic(err)
	}
	exPath := filepath.Dir(ex)
	os.Chdir(exPath)

	if debug {
		print.WriteC(print.Red, "-> Binding... ")
	}

	boundProgram := binder.BindProgram(memberList)

	// any errors after binding? -> die();
	print.CrashIfErrorsFound()

	if debug {
		print.PrintC(print.Green, "Done!")
		boundProgram.Print()
	}

	//boundProgram.Print()
	//boundProgram.PrintStatements()

	return boundProgram, arguments
}

// RunTests runs all the test files in /tests/
func RunTests() {
	files, err := ioutil.ReadDir("tests")
	if err != nil {
		// better error later
		print.PrintC(print.DarkRed, "ERROR: failed reading /tests/ directory!")
	}
	tests := make([]string, 0)
	for _, file := range files {
		if !file.IsDir() && strings.Contains(file.Name(), ".rct") {
			tests = append(tests, file.Name())
		}
	}
	for _, test := range tests {
		print.PrintC(
			print.Cyan,
			fmt.Sprintf("\nTesting test file \"%s\":", test),
		)
		// forgot to actually run the file lol
		go InterpretFile("tests/" + test)

	}
}

// Help shows help message (pretty standard nothing special)
func Help() {
	header := "ReCT Go Compiler v" + currentVersion
	lines := strings.Repeat("-", len(header))

	fmt.Println(lines)
	fmt.Println(header)
	fmt.Println(lines)

	fmt.Print("\nUsage: ")
	print.PrintC(print.Green, "rgoc <file> [options]\n")
	fmt.Println("<file> can be the path to any ReCT file (.rct)")
	fmt.Println("\n[Options]")

	helpSegments := []HelpSegment{
		{"Help", executableName + " -h", "disabled (default)", "Shows this help message!"},
		//{"Interpret", executableName + " -i", "disabled (default)", "Enables interpreter mode, source code will be interpreted instead of compiled."},
		//{"File logging", executableName + " -l", "disabled (default)", "Logs process information in a log file"},
		{"Output", executableName + " -o", "altered source path", "Sets the compiler's output path"},
		{"LLVM", executableName + " --llvm", "disabled (default)", "Output a LLVM Module (.ll) file instead of an executable"},
		{"Optimize", executableName + " -O", "disabled (default)", "Compile the executable with -O2 compiler optimizations enabled"},
		{"Package", executableName + " --package", "none (default)", "Compile as a package with the given name"},
		{"Debug", executableName + " -xx", "disabled (default)", "Shows brief process information in the command line and enable verbose ARC"},
		{"Look up", executableName + " -lookup", "no code (default)", "Shows further detail about errors you may have encountered"},
	}

	p0, p1, p2, p3 := findPaddings(helpSegments)

	for _, segment := range helpSegments {
		segment.Print(p0, p1, p2, p3)
	}

	fmt.Println("")
	print.WriteC(print.Gray, "Still having troubles? Get help on the offical Discord server: ")
	print.WriteCF(print.DarkBlue, "%s!\n", discordInvite) // Moved so link is now blue
}

// Version Shows the current compiler version
func Version() {
	fmt.Println("ReCT Go Compiler")
	fmt.Print("ReCT version: ")
	print.PrintC(print.Blue, currentVersion)
	print.WriteC(print.Gray, "\nStill having troubles? Get help on the offical Discord server: ")
	print.WriteCF(print.DarkBlue, "%s!\n", discordInvite) // Moved so link is now blue
}

type HelpSegment struct {
	Command      string
	Example      string
	DefaultValue string
	Explanation  string
}

func (seg *HelpSegment) Print(p0 int, p1 int, p2 int, p3 int) {
	print.WriteCF(print.Cyan, "%-*s", p0, seg.Command)
	print.WriteC(print.DarkGray, ":")
	print.WriteCF(print.Blue, " %-*s", p1, seg.Example)
	print.WriteC(print.DarkGray, ":")
	print.WriteCF(print.Yellow, " %-*s", p2, seg.DefaultValue)
	print.WriteC(print.DarkGray, ":")
	print.WriteCF(print.Green, " %-*s", p3, seg.Explanation)
	fmt.Println("")
}

func findPaddings(segments []HelpSegment) (int, int, int, int) {
	p0 := 0
	p1 := 0
	p2 := 0
	p3 := 0

	for _, segment := range segments {
		if len(segment.Command) > p0 {
			p0 = len(segment.Command)
		}
		if len(segment.Example) > p1 {
			p1 = len(segment.Example)
		}
		if len(segment.DefaultValue) > p2 {
			p2 = len(segment.DefaultValue)
		}
		if len(segment.Explanation) > p3 {
			p3 = len(segment.Explanation)
		}
	}

	return p0 + 1, p1 + 1, p2 + 1, p3 + 1
}
