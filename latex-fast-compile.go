package main

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
	flag "github.com/spf13/pflag"
)

// the version will be set by goreleaser based on the git tag
var version string = "--"

// Display the usage help message
func printVersion() {
	// get the default error output
	var out = flag.CommandLine.Output()
	// write the help message
	fmt.Fprintf(out, "version: %s\n", version)
	fmt.Fprintf(out, "tex distribution: %s\n", texDistro)
	fmt.Fprintf(out, "pdflatex version: %s\n", texVersionStr)
}

// Display the usage help message
func printHelp() {
	// get the default error output
	var out = flag.CommandLine.Output()
	// write the help message
	fmt.Fprintf(out, "latex-fast-compile (version: %s): compile latex source using precompiled header.\n\n", version)
	fmt.Fprintf(out, "Usage: latex-fast-compile [options] filename[.tex].\n")
	fmt.Fprintf(out, "  If filename.fmt is missing it is build before the compilation.\n")
	fmt.Fprintf(out, "  The available options are:\n\n")
	flag.PrintDefaults()
	fmt.Fprintf(out, "\n")
}

// Check for error
// - do nothing if no error
// - print the error message and panic if there is an error
func check(e error, m ...interface{}) {
	if e != nil {
		if len(m) > 0 {
			fmt.Print("Error: ")
			fmt.Println(m...)
		} else {
			fmt.Println("Error.")
		}
		fmt.Println(e)
		// if we are in watch mode, do not halt on error
		if !isRecompiling {
			panic(e)
		}
	}
}

// This is the last function executed in this program.
func end() {
	// in case of error return status is 1
	if r := recover(); r != nil {
		os.Exit(1)
	}

	// the normal return status is 0
	os.Exit(0)
}

type infoLevelType uint8

const (
	infoNo infoLevelType = iota
	infoErrors
	infoErrorsAndLog
	infoActions
	infoDebug
)

func infoLevelFromString(info string) infoLevelType {
	switch info {
	case "errors":
		return infoErrors
	case "errors+log":
		return infoErrorsAndLog
	case "actions":
		return infoActions
	case "debug":
		fmt.Println("Set info level to debug.")
		return infoDebug
	case "no":
		return infoNo
	default:
		check(errors.New("Invalid info level."))
		return infoDebug
	}
}

var (
	// flags
	mustBuildFormat    bool
	mustCompileAll     bool
	mustNotSync        bool
	synctexOption      string = "--synctex=-1"
	mustNoWatch        bool
	numCompilesAtStart int
	mustShowHelp       bool
	mustShowVersion    bool
	tempFolderName     string
	tempFolderOption   string = ""
	infoLevelFlag      string
	logSanitize        string
	// global variables
	texDistro     string
	texVersionStr string
	basename      string
	formatbase    string
	isRecompiling bool
	infoLevel     infoLevelType
	reSanitize    *regexp.Regexp
	// temp variable for error catch
	err error
)

func getTeXVersion() string {
	// build command
	var cmdOutput strings.Builder
	cmd := exec.Command("pdflatex", "--version")
	cmd.Stdout = &cmdOutput
	cmd.Stderr = &cmdOutput
	// print command?
	if infoLevel == infoDebug {
		fmt.Println(delimit("command", cmd.String()))
	}
	// run command
	err = cmd.Run()
	linesOutput := strings.Split(cmdOutput.String(), "\n")
	if err != nil || len(linesOutput) == 0 {
		return ""
	}

	return strings.TrimSpace(linesOutput[0])
}

func setTeXVersion() {
	texVersionStr = getTeXVersion()
	if strings.Contains(texVersionStr, "MiKTeX") {
		texDistro = "miktex"
	}
	if strings.Contains(texVersionStr, "TeX Live") {
		texDistro = "texlive"
	}
}

// Set the configuration variables from the command line flags
func SetParameters() {
	setTeXVersion()
	flag.BoolVar(&mustBuildFormat, "precompile", false, "Force to create .fmt file even if it exists.")
	flag.BoolVar(&mustCompileAll, "skip-fmt", false, "Skip .fmt file and compile all.")
	flag.BoolVar(&mustNotSync, "no-synctex", false, "Do not build .synctex file.")
	if texDistro == "miktex" {
		flag.StringVar(&tempFolderName, "temp-folder", "temp_files", "Folder to store all temp files, .fmt included.")
	}
	flag.BoolVar(&mustNoWatch, "no-watch", false, "Do not watch for file changes in the .tex file.")
	flag.IntVar(&numCompilesAtStart, "compiles-at-start", 1, "Number of compiles before to start watching.")
	flag.StringVar(&infoLevelFlag, "info", "actions", "The info level [no|errors|errors+log|actions|debug].")
	flag.StringVar(&logSanitize, "log-sanitize", "(?m)^(?:! |l.|<recently read> ).*$", "Match the log against this regex before display, or display all if empty.\n")
	flag.BoolVarP(&mustShowVersion, "version", "v", false, "Print the version number.")
	flag.BoolVarP(&mustShowHelp, "help", "h", false, "Print this help message.")
	// keep the flags order
	flag.CommandLine.SortFlags = false
	// in case of error do not display second time
	flag.CommandLine.Init("latex-fast-compile", flag.ContinueOnError)
	// The help message
	flag.Usage = printHelp
	err = flag.CommandLine.Parse(os.Args[1:])
	// display the help message if the flag is set or if there is an error
	if mustShowHelp || err != nil {
		flag.Usage()
		check(err, "Problem parsing parameters.")
		// if no error
		os.Exit(0)
	}
	setTeXVersion()
	// display the version?
	if mustShowVersion {
		printVersion()
		os.Exit(0)
	}

	// check for positional parameters
	if flag.NArg() > 1 {
		check(errors.New("No more than one positional parameter (.tex filename) can be specified."))
	}
	if flag.NArg() == 0 {
		check(errors.New("You should provide a .tex file to compile."))
	}
	basename = strings.TrimSuffix(flag.Arg(0), ".tex")
	if len(tempFolderName) > 0 {
		tempFolderOption = "--aux-directory=" + tempFolderName
	}

	if mustNotSync {
		synctexOption = ""
	}

	if len(logSanitize) > 0 {
		reSanitize, err = regexp.Compile(logSanitize)
		check(err)
	}

	infoLevel = infoLevelFromString(infoLevelFlag)

	// check if pdflatex is present
	if len(texDistro) == 0 {
		if len(texVersionStr) == 0 {
			check(errors.New("Can't find pdflatex in the current path."))
		} else {
			if infoLevel > infoNo {
				fmt.Println("Unknown pdftex version:", texVersionStr)
			}
		}
	}
	if infoLevel == infoDebug {
		printVersion()
		pathPDFLatex, err := exec.LookPath("pdflatex")
		if err != nil {
			// We should never be here
			check(errors.New("Can't find pdflatex in the current path (bis)."))
		}
		fmt.Println("pdflatex location:", pathPDFLatex)
	}
}

// check if file is missing
func isFileMissing(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return true
	}
	return info.IsDir()
}

func delimit(what, msg string) string {
	var line string = strings.Repeat("-", 77)
	return line + " " + what + "\n" + msg + "\n" + line
}

func sanitizeLog(log []byte) string {

	if reSanitize == nil {
		return delimit("raw log", string(log))
	}

	errorLines := reSanitize.FindAll(log, -1)
	if len(errorLines) == 0 {
		return ("Nothing interesting in the log.")
	} else {
		return delimit("sanitized log", string(bytes.Join(errorLines, []byte("\n"))))
	}

}

// Build, print and run command
func run(info, command string, args ...string) {
	var startTime time.Time
	// build command
	var cmdOutput strings.Builder
	cmd := exec.Command(command, args...)
	cmd.Stdout = &cmdOutput
	cmd.Stderr = &cmdOutput
	// print command?
	if infoLevel == infoDebug {
		fmt.Println(delimit("command", cmd.String()))
	}
	// print action?
	if infoLevel >= infoActions {
		startTime = time.Now()
		fmt.Print(info + "...")
	}
	// run command
	err = cmd.Run()
	// print time?
	if infoLevel >= infoActions {
		fmt.Printf("done [%.1fs]\n", time.Since(startTime).Seconds())
	}
	// if error
	if infoLevel == infoDebug || infoLevel >= infoErrors && err != nil {
		if err != nil {
			fmt.Println("\nThe compilation finished with errors.")
		}
		if infoLevel >= infoErrorsAndLog {
			dat, err := ioutil.ReadFile(filepath.Join(tempFolderName, basename+".log"))
			check(err, "Problem reading ", basename+".log")
			fmt.Println(sanitizeLog(dat))
		}
	}
}

func info(message ...interface{}) {
	if infoLevel >= infoActions {
		fmt.Println(message...)
	}
}

func precompile() {
	if mustBuildFormat || !mustCompileAll && isFileMissing(formatbase+".fmt") {
		run("Precompile", "etex", "-interaction=batchmode", "-halt-on-error", tempFolderOption, "-initialize", "-jobname="+basename, "&pdflatex", "mylatexformat.ltx", basename+".tex")
	}
}

func recompile() {
	if mustCompileAll || isFileMissing(formatbase+".fmt") {
		if !mustCompileAll {
			info("Oops, " + formatbase + ".fmt is missing.")
		}
		run("Compile (skip precompile)", "pdflatex", "-interaction=batchmode", "-halt-on-error", synctexOption, tempFolderOption, basename+".tex")
	} else {
		run("Compile (use precomiled "+formatbase+".fmt)", "pdflatex", "-interaction=batchmode", "-halt-on-error", synctexOption, tempFolderOption, "&"+basename, basename+".tex")
	}
	if infoLevel == infoDebug {
		fmt.Println("End fast compile.")
	}
	if isRecompiling {
		info("Wait for new changes...")
	}
	isRecompiling = false
}

func main() {
	// error handling
	defer end()
	// The flags
	SetParameters()
	// start compiling
	if isFileMissing(basename + ".tex") {
		check(errors.New("file " + basename + ".tex is missing."))
	}

	formatbase = filepath.Join(tempFolderName, basename)
	precompile()

	for i := 0; i < numCompilesAtStart; i++ {
		recompile()
	}

	if !mustNoWatch {
		info("Watching for files changes...(to exit press Ctrl/Cmd-C).")
		// creates a new file watcher
		watcher, err := fsnotify.NewWatcher()
		check(err, "Problem creating the file watcher")
		defer watcher.Close()

		// stop watching ?
		done := make(chan bool)

		// watch and print
		var ok bool
		go func() {
			for {
				select {
				case event, ok := <-watcher.Events:
					if !ok {
						return
					}
					if event.Op&fsnotify.Write == fsnotify.Write {
						if !isRecompiling {
							isRecompiling = true
							info("File changed.")
							go recompile()
						}
					}
				case err, ok = <-watcher.Errors:
					if !ok {
						return
					}
					check(err, "Problem with the file watcher")
				}
			}
		}()

		// out of the box fsnotify can watch a single file, or a single directory
		err = watcher.Add(basename + ".tex")
		check(err, "Problem watching", basename+".tex")

		<-done
	}
}
