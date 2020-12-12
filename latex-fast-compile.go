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
func help() {
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
	default:
		return infoNo
	}
}

var (
	// flags
	mustBuildFormat  bool
	mustCompileAll   bool
	mustNotSync      bool
	synctexOption    string = "--synctex=-1"
	mustWatchFile    bool
	mustWaitForEdit  bool
	mustShowHelp     bool
	tempFolderName   string
	tempFolderOption string = ""
	infoLevelFlag    string
	mustUseRawLog    bool
	// global variables
	basename      string
	formatbase    string
	isRecompiling bool
	infoLevel     infoLevelType
	// temp variable for error catch
	err error
)

// Set the configuration variables from the command line flags
func SetParameters() {
	flag.BoolVar(&mustBuildFormat, "precompile", false, "Force to create .fmt file even if it exists.")
	flag.BoolVar(&mustCompileAll, "skip-fmt", false, "Skip .fmt file and compile all.")
	flag.BoolVar(&mustNotSync, "no-synctex", false, "Do not build .synctex file.")
	flag.StringVar(&tempFolderName, "temp-folder", "", "Folder to store all temp files, .fmt included.")
	flag.BoolVar(&mustWatchFile, "watch", false, "Keep watching the .tex file and recompile if changed.")
	flag.BoolVar(&mustWaitForEdit, "wait-modify", false, "Do not compile before the first file modification (needs --watch).")
	flag.StringVar(&infoLevelFlag, "info", "actions", "The info level [no|errors|errors+log|actions|debug].")
	flag.BoolVar(&mustUseRawLog, "raw-log", false, "Display raw log in case of error.")
	flag.BoolVarP(&mustShowHelp, "help", "h", false, "Print this help message.")
	// keep the flags order
	flag.CommandLine.SortFlags = false
	// in case of error do not display second time
	flag.CommandLine.Init("latex-fast-compile", flag.ContinueOnError)
	// The help message
	flag.Usage = help
	err = flag.CommandLine.Parse(os.Args[1:])
	// display the help message if the flag is set or if there is an error
	if mustShowHelp || err != nil {
		flag.Usage()
		check(err, "Problem parsing parameters.")
		// if no error
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

	infoLevel = infoLevelFromString(infoLevelFlag)
}

// check if file is missing
func isFileMissing(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return true
	}
	return info.IsDir()
}

func sanitizeLog(log []byte) string {
	re := regexp.MustCompile(`(?m)^(?:! |l.).*$`)
	errorLines := re.FindAll(log, -1)
	return string(bytes.Join(errorLines, []byte("\n"))) + "\n"
}

// Build, print and run command
func run(info, command string, args ...string) {
	var startTime time.Time
	var line string = strings.Repeat("-", 77)
	// build command
	var cmdOutput strings.Builder
	cmd := exec.Command(command, args...)
	cmd.Stdout = &cmdOutput
	cmd.Stderr = &cmdOutput
	// print command?
	if infoLevel == infoDebug {
		fmt.Println(line)
		fmt.Println(cmd.String())
		fmt.Println(line)
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
		fmt.Printf("done[%.1fs]\n", time.Since(startTime).Seconds())
	}
	// Display the latex output?
	if infoLevel == infoDebug {
		fmt.Println(cmdOutput.String())
	}
	// if error
	if infoLevel == infoDebug || infoLevel >= infoErrors && err != nil {
		if err != nil {
			fmt.Println("\nThe compilation finished with errors.")
		}
		if infoLevel != infoDebug && infoLevel >= infoErrorsAndLog && err != nil {
			dat, err := ioutil.ReadFile(filepath.Join(tempFolderName, basename+".log"))
			check(err, "Problem reading ", basename+".log")
			fmt.Println(line)
			if mustUseRawLog {
				fmt.Print(string(dat))
			} else {
				fmt.Print(sanitizeLog(dat))
			}
			fmt.Println(line)
		}
	}
}

func info(message ...interface{}) {
	if infoLevel >= infoActions {
		fmt.Println(message...)
	}
}

func recompile() {
	interactionOption := "-interaction=batchmode"
	if infoLevel == infoDebug {
		interactionOption = "-interaction=nonstopmode"
	}
	if !mustCompileAll && (mustBuildFormat && !isRecompiling || isFileMissing(formatbase+".fmt")) {
		run("Precompile", "etex", interactionOption, "-halt-on-error", tempFolderOption, "-initialize", "-jobname="+basename, "&pdflatex", "mylatexformat.ltx", basename+".tex")
	}

	if mustCompileAll || isFileMissing(formatbase+".fmt") {
		if !mustCompileAll {
			info("Oops, " + formatbase + ".fmt still missing.")
		}
		run("Compile (skip precompile)", "pdflatex", interactionOption, "-halt-on-error", synctexOption, tempFolderOption, basename+".tex")
	} else {
		run("Compile (use precomiled "+formatbase+".fmt)", "pdflatex", interactionOption, "-halt-on-error", synctexOption, tempFolderOption, "&"+basename, basename+".tex")
	}
	if infoLevel == infoDebug {
		fmt.Println(strings.Repeat("=", 77))
		fmt.Println("End fast compile.")
	}
	if isRecompiling {
		info("Wait for new changes ...")
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

	if !mustWatchFile || !mustWaitForEdit {
		recompile()
	}

	if mustWatchFile {
		info("Watching for files changes ... (to exit press Ctrl/Cmd-C).")
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
