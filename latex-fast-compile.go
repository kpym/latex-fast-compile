package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"time"
	"unicode"

	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"

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
	fmt.Fprintf(out, "pdftex version: %s\n", texVersionStr)
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

// the infoLevel type and constants
type infoLevelType uint8

const (
	infoNo infoLevelType = iota
	infoErrors
	infoErrorsAndLog
	infoActions
	infoDebug
)

// convert the flag `--info` flag to the corresponding level.
func infoLevelFromString(info string) infoLevelType {
	switch info {
	case "no":
		return infoNo
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
		check(errors.New("Invalid info level."))
		return infoDebug
	}
}

var (
	// flags
	mustBuildFormat    bool
	mustCompileAll     bool
	mustNotSync        bool
	mustNoWatch        bool
	numCompilesAtStart int
	mustShowHelp       bool
	mustShowVersion    bool
	infoLevelFlag      string
	logSanitize        string
	splitPattern       string
	tempFolderName     string
	clearFlag          string
	mustClear          bool
	auxExtensions      string
	mustNoNormalize    bool
	// global variables
	texDistro         string
	texVersionStr     string
	inBaseOriginal    string
	inBase            string
	outBase           string
	isRecompiling     bool
	infoLevel         infoLevelType
	reSanitize        *regexp.Regexp
	reSplit           *regexp.Regexp
	precompileOptions []string
	compileOptions    []string
	// temp variable for error catch
	err error
)

// getTeXVersion return the first line from `pdftex --version`
func getTeXVersion() string {
	// build command
	var cmdOutput strings.Builder
	cmd := exec.Command("pdftex", "--version")
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

// Try to recognize the distribution based on the pdftex version.
func setDistro() {
	texVersionStr = getTeXVersion()
	if strings.Contains(texVersionStr, "MiKTeX") {
		texDistro = "miktex"
	}
	if strings.Contains(texVersionStr, "TeX Live") {
		texDistro = "texlive"
	}

	precompileOptions = []string{"-interaction=batchmode", "-halt-on-error", "-ini"}
	compileOptions = []string{"-interaction=batchmode", "-halt-on-error"}
}

// used in normalizeName
func isMn(r rune) bool {
	return unicode.Is(unicode.Mn, r) // Mn: nonspacing marks
}

// normalizeName remove accents and spaces
// borrowed from https://stackoverflow.com/a/26722698
func normalizeName(fileName string) string {
	t := transform.Chain(norm.NFD, transform.RemoveFunc(isMn), norm.NFC)
	result, _, _ := transform.String(t, fileName)
	return strings.ReplaceAll(result, " ", "")
}

// Set the configuration variables from the command line flags
func SetParameters() {
	// set the distro based on the pdftex version
	setDistro()
	// the list of flags
	flag.BoolVar(&mustBuildFormat, "precompile", false, "Force to create .fmt file even if it exists.")
	flag.BoolVar(&mustCompileAll, "skip-fmt", false, "Skip .fmt file and compile all.")
	flag.BoolVar(&mustNotSync, "no-synctex", false, "Do not build .synctex file.")
	flag.BoolVar(&mustNoWatch, "no-watch", false, "Do not watch for file changes in the .tex file.")
	flag.IntVar(&numCompilesAtStart, "compiles-at-start", 1, "Number of compiles before to start watching.")
	flag.StringVar(&infoLevelFlag, "info", "actions", "The info level [no|errors|errors+log|actions|debug].")
	flag.StringVar(&logSanitize, "log-sanitize", `(?m)^(?:! |l\.|<recently read> ).*$`, "Match the log against this regex before display, or display all if empty.\n")
	flag.StringVar(&splitPattern, "split", `(?m)^\s*(?:%\s*end\s*preamble|\\begin{document})`, "Match the log against this regex before display, or display all if empty.\n")
	flag.StringVar(&tempFolderName, "temp-folder", "", "Folder to store all temp files, .fmt included.")
	flag.StringVar(&clearFlag, "clear", "auto", "Clear auxiliary files and .fmt at end [auto|yes|no].\n When watching auto=true, else auto=false.\nIn debug mode clear is false.")
	flag.StringVar(&auxExtensions, "aux-extensions", "aux,bbl,blg,fmt,fff,glg,glo,gls,idx,ilg,ind,lof,lot,nav,out,ptc,snm,sta,stp,toc", "Extensions to remove in clear at the end procedure.\n")
	flag.BoolVar(&mustNoNormalize, "no-normalize", false, "Keep accents and spaces in intermediate files.")
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
	// set the info level
	infoLevel = infoLevelFromString(infoLevelFlag)
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

	inBaseOriginal = strings.TrimSuffix(flag.Arg(0), ".tex")
	if mustNoNormalize {
		inBase = inBaseOriginal
	} else {
		inBase = normalizeName(inBaseOriginal)
	}

	// synctex or not?
	if !mustNotSync {
		compileOptions = append(compileOptions, "--synctex=-1")
	}

	// sanitize log or not?
	if len(logSanitize) > 0 {
		reSanitize, err = regexp.Compile(logSanitize)
		check(err)
	}

	// check if pdftex is present
	if len(texDistro) == 0 {
		if len(texVersionStr) == 0 {
			check(errors.New("Can't find pdftex in the current path."))
		} else {
			if infoLevel > infoNo {
				fmt.Println("Unknown pdftex version:", texVersionStr)
			}
		}
	}
	if infoLevel == infoDebug {
		printVersion()
		pathPDFLatex, err := exec.LookPath("pdftex")
		if err != nil {
			// We should never be here
			check(errors.New("Can't find pdftex in the current path (bis)."))
		}
		fmt.Println("pdftex location:", pathPDFLatex)
	}

	// set split pattern
	if len(splitPattern) > 0 {
		reSplit, err = regexp.Compile(splitPattern)
		check(err)
	} else {
		mustCompileAll = true
	}
	// set temp folder?
	if !mustNoNormalize {
		tempFolderName = normalizeName(tempFolderName)
	}
	if len(tempFolderName) > 0 {
		if inBase == inBaseOriginal && texDistro == "miktex" {
			precompileOptions = append(precompileOptions, "-aux-directory="+tempFolderName)
			compileOptions = append(compileOptions, "-aux-directory="+tempFolderName)
		} else {
			precompileOptions = append(precompileOptions, "-output-directory="+tempFolderName)
			compileOptions = append(compileOptions, "-output-directory="+tempFolderName)
		}
		outBase = filepath.Join(tempFolderName, inBase)
	} else {
		outBase = inBase
	}

	// set the source filename
	precompileName := "&pdflatex " + inBase + ".preamble.tex"
	precompileOptions = append(precompileOptions, "-jobname="+inBase, precompileName)
	compileName := "&" + inBase + " " + inBase + ".body.tex"
	if mustCompileAll {
		compileName = "&pdflatex " + inBase + ".tex"
	}
	compileOptions = append(compileOptions, "-jobname="+inBase, compileName)

	// clear or not
	mustClear = (infoLevel < infoDebug) && (clearFlag == "yes" || clearFlag == "auto" && !mustNoWatch)
}

// check if file is missing
func isFileMissing(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return true
	}
	return info.IsDir()
}

// check if file is missing
func isFolderMissing(foldername string) bool {
	info, err := os.Stat(foldername)
	return err != nil || !info.IsDir()
}

// delimit produce something like
// ---------------------- what
// msg
// ----------------------
// and is used to delimit log output and commands when debugging
func delimit(what, msg string) string {
	var line string = strings.Repeat("-", 77)
	return line + " " + what + "\n" + msg + "\n" + line
}

// sanitizeLog try to keep only the lines related to the errors.
// It is controlled by the regular expression set in `--log-sanitize`.
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

// Build, print and run command.
// The info parameter is printed if the infoLevel authorize this.
func run(info, command string, args ...string) {
	var startTime time.Time
	// build command (without possible interactions)
	cmd := exec.Command(command, args...)
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil
	// print command?
	if infoLevel == infoDebug {
		fmt.Println(delimit("command", cmd.String()))
	}
	// print action?
	if infoLevel >= infoActions {
		startTime = time.Now()
		fmt.Print("::::::: ", info+"...")
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
			dat, err := ioutil.ReadFile(outBase + ".log")
			check(err, "Problem reading ", outBase+".log")
			fmt.Println(sanitizeLog(dat))
		}
	}
}

// info print the message only if the infoLevel authorize it.
func info(message ...interface{}) {
	if infoLevel >= infoActions {
		fmt.Println(message...)
	}
}

// Borrowed from https://stackoverflow.com/a/21067803
func copyFile(src, dst string) (ok bool) {
	defer func() {
		if err == nil {
			ok = true
		} else {
			check(errors.New("Error while copy " + src + " to " + dst + "."))
		}
	}()

	info(" copy", src, "to", dst)

	in, err := os.Open(src)
	if err != nil {
		return
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return
	}
	defer func() {
		cerr := out.Close()
		if err == nil {
			err = cerr
		}
	}()
	if _, err = io.Copy(out, in); err != nil {
		return
	}
	err = out.Sync()
	return
}

// splitTeX split the `.tex` file to two files `.preamble.tex` and `.body.tex`.
// it also append `\dump` to the preamble and perpend `%&...` to the body.
// both files are saved in the same folder (not in the temporary one) as the original source.
func splitTeX() (ok bool) {
	sourceName := inBaseOriginal + ".tex"
	if isFileMissing(sourceName) {
		check(errors.New("File " + sourceName + " is missing."))
	}
	// we hope that...
	ok = true
	// copy the original?
	if mustCompileAll && inBaseOriginal != inBase {
		ok = copyFile(inBaseOriginal+".tex", inBase+".tex")
	}
	// is the split necessary?
	if !mustBuildFormat && mustCompileAll {
		return
	}
	// read the file
	var texdata []byte
	for i := 0; i < 2; i++ {
		texdata, err = ioutil.ReadFile(sourceName)
		check(err, "Problem reading "+sourceName+" for splitting.")
		if len(texdata) == 0 {
			if i == 0 {
				info("Problem reading " + sourceName + " for splitting. Try one more time.")
				time.Sleep(100 * time.Millisecond)
			} else {
				check(errors.New("Problem reading " + sourceName + " for splitting."))
				return false
			}
		} else {
			break
		}
	}
	// split the file
	loc := reSplit.FindIndex(texdata)
	if len(loc) == 0 {
		check(errors.New("Problem while splitting " + sourceName + " to preamble and body."))
		return false
	}
	// important to first get body because textdata is polluted by \dump in the next line
	bodyName := inBase + ".body.tex"
	info(" create", bodyName)
	texBody := append([]byte("\n%&"+inBase+"\n"), texdata[loc[0]:]...)
	err = ioutil.WriteFile(bodyName, texBody, 0644)
	check(err, "Problem while writing", bodyName)
	ok = (err == nil)

	preambleName := inBase + ".preamble.tex"
	info(" create", preambleName)
	texPreamble := append(texdata[:loc[0]], []byte("\\dump")...)
	err = ioutil.WriteFile(preambleName, texPreamble, 0644)
	check(err, "Problem while writing", preambleName)
	ok = ok && (err == nil)

	return ok
}

// clearFiles is used by clearTeX and clearAux.
// Given one base and multiple extensions it removes the corresponding files.
func clearFiles(base, extensions string) {
	for _, ext := range strings.Split(extensions, ",") {
		fileToDelete := base + "." + strings.TrimSpace(ext)
		if isFileMissing(fileToDelete) {
			continue
		}
		if infoLevel >= infoActions {
			info(" remove", fileToDelete)
		}
		os.Remove(fileToDelete)
	}
}

// clear the files produced by splitTeX().
func clearTeX() {
	clearFiles(inBase, "preamble.tex,body.tex")
}

// clear the auxiliary files produced by pdftex
func clearAux() {
	clearFiles(outBase, auxExtensions)
}

// precompile produce the `.fmt` file based on the `.preamble.tex` part.
func precompile() {
	if mustBuildFormat || !mustCompileAll && isFileMissing(outBase+".fmt") {
		run("Precompile", "pdftex", precompileOptions...)
	}
	// we tel to splitTex that the preamble is not needed any more
	mustBuildFormat = false
}

// compile produce the `.pdf` file based on the `.body.tex` part.
func compile(draft bool) {
	msg := "Compile "
	if draft {
		msg += "draft "
	}
	if mustCompileAll {
		msg += "(skip precompile)"
	} else {
		msg += "(use precomiled " + outBase + ".fmt)"
	}
	if draft {
		draftOptions := append(compileOptions, "-draftmode")
		run(msg, "pdftex", draftOptions...)
	} else {
		run(msg, "pdftex", compileOptions...)
	}
	// move/rename .pdf and .synctex to the original source
	if !draft && inBaseOriginal != outBase && (texDistro != "miktex" || inBaseOriginal != inBase) {
		if !isFileMissing(outBase + ".pdf") {
			if copyFile(outBase+".pdf", inBaseOriginal+".pdf") {
				info(" delete", outBase+".pdf")
				os.Remove(outBase + ".pdf")
			}
		}
		if !mustNotSync && !isFileMissing(outBase+".synctex") {
			info(" move", outBase+".synctex", "to", inBaseOriginal+".synctex")
			err = os.Rename(outBase+".synctex", inBaseOriginal+".synctex")
			check(err, "Error while copy "+outBase+".synctex  to "+inBaseOriginal+".synctex.")
		}
	}

	if isRecompiling {
		info("Wait for new changes...")
	}
	isRecompiling = false
}

// recompile is called when the source file changes (and we are watching it).
func recompile() {
	if splitTeX() {
		compile(false)
	} else {
		isRecompiling = false
	}
}

// This is the last function executed in this program.
func end() {
	// clear the files?
	if mustClear {
		clearAux()
	}
	if infoLevel < infoDebug {
		clearTeX()
	} else {
		fmt.Println("Do not clear", inBase+".preamble.tex", "and", inBase+".body.tex.")
		fmt.Println("End.")
	}
	// in case of error return status is 1
	if r := recover(); r != nil {
		os.Exit(1)
	}

	// the normal return status is 0
	os.Exit(0)
}

// If we terminate with Ctrl/Cmd-C we call end()
func catchCtrlC() {
	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		end()
	}()
}

// Ready to go!
func main() {
	// error handling
	catchCtrlC()
	defer end()
	// The flags
	SetParameters()
	// prepare the source files
	splitTeX()

	// create .fmt (if needed)
	precompile()
	// start compiling
	for i := 0; i < numCompilesAtStart; i++ {
		compile(i < numCompilesAtStart-1) // only the last compile is not in draft mode
	}
	// watching ?
	if !mustNoWatch {
		info("Watching for file changes...(to exit press Ctrl/Cmd-C).")
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
							// wait before to start compile
							// hoping that this is enough for the file to be closed before.
							time.AfterFunc(10*time.Millisecond, recompile)
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
		err = watcher.Add(inBaseOriginal + ".tex")
		check(err, "Problem watching", inBaseOriginal+".tex")

		<-done
	}
}
