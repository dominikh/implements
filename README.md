# implements

Eventually, this will become a library/tool/website to check what
interfaces a type implements.

Right now, however, it's only a very preliminary prototype that will
parse a couple of packages from the standard library and list which
types implement which interfaces.

## Install

    go get honnef.co/go/implements

## Usage

`implements -help` to get a description of the use flags.

Currently there are only two flags, `-own` and `-universe`. `-own`
describes for which packages you want to print the "X implements Y"
relationship. `-universe` describes from which packages you want to
consider interfaces.

Both `-own` and `-universe` support the pseudo package "std" which
includes the entire standard library and `...` pattern matching as
used by the Go tools.

For example, to check which types from the entire standard library
implement interfaces from io, you'd run `implements -universe io -own
std`.

## But…

Yes, there are potentially ℙ(M) unique interfaces (all combinations of
method signatures), and an unlimited amount of not-unique named and
unnamed interfaces. That, however, isn't the scope of this tool. This
is more of a "what types _that I care about_ implement io.Reader?" or
a "does my type really implement http.File?" – This would be
especially useful for early discovery of the standard libraries,
enriching Go documentation and assisting editors and IDEs in providing
live feedback and possibly auto completion.

Again, the idea is not to run this unconditionally on all code there
is, but on for example the standard library and specific
packages/types you care about.

## TODO

This is still a prototype. In particular, the following changes have
to be made:

- Cleaner code. Right now it's a mixture of stuff put together until
  it works
- Reverse check: By what types are interfaces implemented?
- Filtering to specific types from packages
- The names of flags will probably change as more are added
