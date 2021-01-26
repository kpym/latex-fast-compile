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
  -x, --xelatex                 Use xelatex in place of pdflatex.
      --compiles-at-start int   Number of compiles before to start watching. (default 1)
      --info string             The info level [no|errors|errors+log|actions|debug]. (default "actions")
      --log-sanitize string     Match the log against this regex before display, or display all if empty.
                                 (default "(?ms)^(?:! |l\\.|<recently read> ).*?$(?:\\s^.*?$){0,2}")
      --split string            The regex that defines the end of the preamble.
                                 (default "(?m)^\\s*(?:%\\s*end\\s*preamble|\\\\begin{document})")
      --temp-folder string      Folder to store all temp files, .fmt included.
      --clear string            Clear auxiliary files and .fmt at end [auto|yes|no].
                                 When watching auto=true, else auto=false.
                                In debug mode clear is false. (default "auto")
      --aux-extensions string   Extensions to remove in clear at the end procedure.
                                 (default "aux,bbl,blg,fmt,fff,glg,glo,gls,idx,ilg,ind,lof,lot,nav,out,ptc,snm,sta,stp,toc")
      --no-normalize            Keep accents and spaces in intermediate file names.
  -v, --version                 Print the version number.
  -h, --help                    Print this help message.
```

## Example

To compile `cylinder.tex` you can simply use:

```bash
> latex-fast-compile cylinder.tex
 create cylinder.body.tex
 create cylinder.preamble.tex
::::::: Precompile...done [2.0s]
::::::: Compile (use precompiled cylinder.fmt)...done [1.1s]
Watching for file changes...(to exit press Ctrl/Cmd-C).
```
1. The source `cylinder.tex` is split to `cylinder.preamble.tex` and `cylinder.body.tex`.
1. Then if the precompiled header is missing (`cylinder.fmt` is missing in our case) it is precompiled from `cylinder.preamble.tex`.
1. The file is compiled using this precompiled header (`cylinder.fmt` in our case) from `cylinder.body.tex`.
1. The program waits (except if `--no-watch` is used) for new changes in the `.tex` file. At every change the source is re-split and the body part is re-compiled using the precompiled header.

### How it works

The `.tex` file is split into two files `.preamble.tex` and `.body.tex`. The file is split at `% end preamble` comment or at `\begin{document}` (which comes first). The file `.preamble.tex` is precompiled to `.fmt` only if needed. The file `.body.tex` is compiled using this `.fmt` file to `.pdf`.

The split point is controlled by the regular expression defined in the `--split` flag. This regular expression follows the [go re2 syntax](https://github.com/google/re2/wiki/Syntax).

### Printed information

The output information is controlled by the string flags `--info` and `--log-sanitize`. The regular expression set in `--log-sanitize`, used to sanitize the log file, follows the [go re2 syntax](https://github.com/google/re2/wiki/Syntax).

### Temp folder

To keep your folder clean of temporary files, precompiled `.fmt` included, a temp folder can be set with the `--temp-folders` flag.
In the case of MiKTeX `-aux-directory` is used, but in TeX Live this option is not available so `-output-directory` is used, but then the resulting `pdf` and the corresponding `synctex` should be moved back to the main folder.

### Bizarre file names

If the filename has non ascii symbols and/or spaces, it is normalized (except if `-no-normalize` is used). For example `Très étrange.tex` will be normalized to `Tresetrange.tex` and at the end the resulting `Tresetrange.pdf` will be renamed back to `Très étrange.pdf`.

```bash
latex-fast-compile Très\ étrange.tex --no-watch --compiles-at-start=2
 create Tresetrange.body.tex
 create Tresetrange.preamble.tex
::::::: Precompile...done [3.3s]
::::::: Compile draft (use precompiled Tresetrange.fmt)...done [1.9s]
::::::: Compile (use precompiled Tresetrange.fmt)...done [2.3s]
 copy Tresetrange.pdf to Très étrange.pdf
 delete Tresetrange.pdf
 move Tresetrange.synctex to Très étrange.synctex
 remove Tresetrange.preamble.tex
 remove Tresetrange.body.tex
```
This is necessary because this kind of filenames do not work well for precompiled `.fmt` files.

### XeLaTex

We can use `xelatex` in place of `pdflatex` by specifying the `-x` (`--xelatex`) option. But it is good to know that `fontspec` and `polyglossia` (and any other package that access `ttf` or `otf` fonts) can't be in the precompiled header. If these two libraries are present in the preamble they are moved outside. But if they are included indirectly, the compilation will fail.

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
