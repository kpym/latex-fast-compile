# latex-fast-compile

A small executable that use `mylatexformat` to precompile the header and speed up next compilations.

## Usage

```bash
> latex-fast-compile -h
latex-fast-compile (version: --): compile latex source using precompiled header.

Usage: latex-fast-compile [options] filename[.tex].
  If filename.fmt is missing it is build before the compilation.
  The available options are:

      --precompile           Force to create .fmt file even if it exists.
      --skip-fmt             Skip .fmt file and compile all.
      --no-synctex           Do not build .synctex file.
      --temp-folder string   Folder to store all temp files, .fmt included.
      --watch                Keep watching the .tex file and recompile if changed.
      --wait-modify          Do not compile before the first file modification (needs --watch).
      --info string          The info level [no|errors|errors+log|actions|debug]. (default "actions")
      --raw-log              Display raw log in case of error.
  -h, --help                 Print this help message.
```

## Example

If we want to compile `cylinder.tex` and the file `cylinder.fmt` is missing:

```bash
> latex-fast-compile cylinder

precompile ...
-----------------------------------------------------------------------------
etex.exe -interaction=batchmode -halt-on-error -initialize -jobname=cylinder &pdflatex mylatexformat.ltx cylinder.tex
-----------------------------------------------------------------------------
This is pdfTeX, Version 3.14159265-2.6-1.40.21 (MiKTeX 20.11) (INITEX)
entering extended mode

Use precomiled cylinder.fmt
-----------------------------------------------------------------------------
pdflatex.exe -interaction=batchmode -halt-on-error --synctex=-1  &cylinder cylinder.tex
-----------------------------------------------------------------------------
This is pdfTeX, Version 3.14159265-2.6-1.40.21 (MiKTeX 20.11)
entering extended mode
=============================================================================
End fast compile.
```

And if you run it second time:

```bash
> latex-fast-compile cylinder

Use precomiled cylinder.fmt
-----------------------------------------------------------------------------
pdflatex.exe -interaction=batchmode -halt-on-error --synctex=-1  &cylinder cylinder.tex
-----------------------------------------------------------------------------
This is pdfTeX, Version 3.14159265-2.6-1.40.21 (MiKTeX 20.11)
entering extended mode
=============================================================================
End fast compile.
```

## Installation

### Precompiled executables

You can download the executable for your platform from the [Realases](https://github.com/kpym/latex-fast-compile/releases).

### Compile it yourself

#### Using Go

This method will comile to executable named `latex-fast-compile`.

```shell
$ go get github.com/kpym/latex-fast-compile
```

#### Using goreleaser

After cloning this repo you can comile the sources with [goreleaser](https://github.com/goreleaser/goreleaser/) for all available platforms:

```shell
git clone https://github.com/kpym/latex-fast-compile.git .
goreleaser --snapshot --skip-publish --rm-dist
```

You will find the resulting binaries in the `dist/` subfolder.

## License

MIT
