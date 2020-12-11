package main

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

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
		fmt.Printf("More info: %v\n", e)
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

var (
	// flags
	isDirtyFormat    bool
	isDirectSource   bool
	noSync           bool
	synctexOption    string = "--synctex=-1"
	watchFile        bool
	waitForEdit      bool
	showhelp         bool
	tempFolderName   string
	tempFolderOption string = ""
	// global variables
	basename      string
	formatbase    string
	isRecompiling bool
	// temp variable for error catch
	err error
)

// Set the configuration variables from the command line flags
func SetParameters() {
	flag.BoolVar(&isDirtyFormat, "precompile", false, "Force to create .fmt file even if it exists.")
	flag.BoolVar(&isDirectSource, "skip-fmt", false, "Skip .fmt file and compile all.")
	flag.BoolVar(&noSync, "no-synctex", false, "Do not build .synctex file.")
	flag.StringVar(&tempFolderName, "temp-folder", "", "Folder to store all temp files, .fmt included.")
	flag.BoolVar(&watchFile, "watch", false, "Keep watching the .tex file and recompile if changed.")
	flag.BoolVar(&waitForEdit, "wait-modify", false, "Do not compile before the first file modification (needs --watch).")
	flag.BoolVarP(&showhelp, "help", "h", false, "Print this help message.")
	// keep the flags order
	flag.CommandLine.SortFlags = false
	// in case of error do not display second time
	flag.CommandLine.Init("latex-fast-compile", flag.ContinueOnError)
	// The help message
	flag.Usage = help
	err = flag.CommandLine.Parse(os.Args[1:])
	// display the help message if the flag is set or if there is an error
	if showhelp || err != nil {
		flag.Usage()
		check(err, "Problem parsing parameters.")
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

	if noSync {
		synctexOption = ""
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

// Build, print and run command
func run(name string, arg ...string) {
	line := strings.Repeat("-", 77)
	// build command
	cmd := exec.Command(name, arg...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stdout
	// print command
	fmt.Println(line)
	fmt.Println(cmd.String())
	fmt.Println(line)
	// run command
	err = cmd.Run()
	if err != nil {
		fmt.Println("\nThe compilation finished with errors.")
		dat, err := ioutil.ReadFile(filepath.Join(tempFolderName, basename+".log"))
		check(err, "Problem reading ", basename+".log")
		fmt.Print(string(dat))
	}
}

func recompile() {
	if !isDirectSource && isDirtyFormat || isFileMissing(formatbase+".fmt") {
		fmt.Println("\nprecompile ...")
		run("etex", "-interaction=batchmode", "-halt-on-error", tempFolderOption, "-initialize", "-jobname="+basename, "&pdflatex", "mylatexformat.ltx", basename+".tex")
	}

	if isDirectSource || isFileMissing(formatbase+".fmt") {
		if !isDirectSource {
			fmt.Println("Oops, " + formatbase + ".fmt still missing.")
		}
		fmt.Println("\nCompile all (skip precompile)")
		run("pdflatex", "-interaction=batchmode", "-halt-on-error", synctexOption, tempFolderOption, basename+".tex")
	} else {
		fmt.Println("\nUse precomiled " + formatbase + ".fmt")
		run("pdflatex", "-interaction=batchmode", "-halt-on-error", synctexOption, tempFolderOption, "&"+basename, basename+".tex")
	}
	fmt.Println(strings.Repeat("=", 77))

	fmt.Print("End fast compile.")
	if isRecompiling {
		fmt.Println(" Wait for new changes ...")
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
		fmt.Println("file " + basename + ".tex is missing.")
		os.Exit(1)
	}

	formatbase = filepath.Join(tempFolderName, basename)

	if !watchFile || !waitForEdit {
		recompile()
	}

	if watchFile {
		fmt.Println("Watching for files changes ... (to exit press Ctrl/Cmd-C).")
		// creates a new file watcher
		watcher, err := fsnotify.NewWatcher()
		check(err, "Problem with the file watcher")
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
							fmt.Println("\nFile changed: recompiling ...")
							go recompile()
						}
					}
				case err, ok = <-watcher.Errors:
					if !ok {
						return
					}
					fmt.Println("error [fsnotify]:", err)
				}
			}
		}()

		// out of the box fsnotify can watch a single file, or a single directory
		err = watcher.Add(basename + ".tex")
		check(err, "Problem watching", basename+".tex")

		<-done
	}
}
