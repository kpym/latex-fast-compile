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
> latex-fast-compile cylinder.tex
Precompile...done [1.6s]
Compile (use precomiled cylinder.fmt)...done [0.8s]
```

And if you run it second time:

```bash
> latex-fast-compile cylinder.tex
Compile (use precomiled cylinder.fmt)...done [0.7s]
```

### watch

You can start watching a file for modifications. In this case every time the file is changed it is recompiled.

```bash
> latex-fast-compile --watch cylinder.tex
Precompile...done [1.5s]
Compile (use precomiled cylinder.fmt)...done [0.7s]
Watching for files changes ... (to exit press Ctrl/Cmd-C).
File changed.
Compile (use precomiled cylinder.fmt)...done [0.7s]
Wait for new changes ...
```

### temps files

If you want to keep your folder clean of temporary files, to put them in `temp_files` subfolder you can use `--temp-folders=temp_files`.

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
