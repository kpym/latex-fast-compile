# latex-fast-compile

A small executable that pre-compile the preamble to speed up future compilations (with `pdflatex`). Then, it watches for file changes, and automatically recompile the source `.tex` file using this precompiled preamble.

## Usage

```bash
> latex-fast-compile -h
latex-fast-compile (version: --): compile latex source using precompiled header.

Usage: latex-fast-compile [options] filename[.tex].
  If filename.fmt is missing it is build before the compilation.
  The available options are:

      --precompile              Force to create .fmt file even if it exists.
      --skip-fmt                Skip .fmt file and compile all.
      --no-synctex              Do not build .synctex file.
      --no-watch                Do not watch for file changes in the .tex file.
      --compiles-at-start int   Number of compiles before to start watching. (default 1)
      --info string             The info level [no|errors|errors+log|actions|debug]. (default "actions")
      --log-sanitize string     Match the log against this regex before display, or display all if empty.
                                 (default "(?m)^(?:! |l\\.|<recently read> ).*$")
      --split string            Match the log against this regex before display, or display all if empty.
                                 (default "(?m)^\\s*(?:%\\s*end\\s*preamble|\\\\begin{document})\\s*$")
      --temp-folder string      Folder to store all temp files, .fmt included [MikTeX only]. (default "temp_files")
  -v, --version                 Print the version number.
  -h, --help                    Print this help message.
```

## Example

To compile `cylinder.tex` you can simply use:

```bash
> latex-fast-compile cylinder.tex
Precompile...done [1.4s]
Compile (use precompiled cylinder.fmt)...done [0.7s]
Watching for files changes...(to exit press Ctrl/Cmd-C).
```

1. First if the precompiled header is missing (`cylinder.fmt` is missing in our case) the header is precompiled.
2. The file is compiled using this precompiled header (`cylinder.fmt` in our case).
3. The program waits (except if `--no-watch` is used) for new changes in the `.tex` file. At every change the source is recompiled using the precompiled header.

### How it works

The `.tex` file is split into two files `.preamble.tex` and `.body.tex`. The file is split at `% end preamble` comment or at `\begin{document}` (which comes first). The file `.preamble.tex` is precompiled to `.fmt` only if needed. The file `.body.tex` is compiled using this `.fmt` file to `.pdf`.

The split point is controlled by the regular expression defined in the `--split` flag. This regular expression follows the [go re2 syntax](https://github.com/google/re2/wiki/Syntax).

### Printed information

The output information is controlled by the string flags `--info` and `--log-sanitize`. The regular expression set in `--log-sanitize`, used to sanitize the log file, follows the [go re2 syntax](https://github.com/google/re2/wiki/Syntax).

### MikTeX specific

To keep your folder clean of temporary files, precompiled `.fmt` included, the subfolder `temp_files` is useed by default. This can be changed with `--temp-folders` flag.

## Installation

### Precompiled executables

You can download the executable for your platform from the [releases](https://github.com/kpym/latex-fast-compile/releases).

### Compile it yourself

#### Using Go

This method will compile to executable named `latex-fast-compile`.

```shell
$ go get github.com/kpym/latex-fast-compile
```

#### Using goreleaser

After cloning this repo you can compile the sources with [goreleaser](https://github.com/goreleaser/goreleaser/) for all available platforms:

```shell
git clone https://github.com/kpym/latex-fast-compile.git .
goreleaser --snapshot --skip-publish --rm-dist
```

You will find the resulting binaries in the `dist/` sub-folder.

## License

MIT
