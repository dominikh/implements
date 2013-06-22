# implements

Eventually, this will become a library/tool/website to check what
interfaces a type implements.

Right now, however, it's only a very preliminary prototype that will
parse a couple of packages from the standard library and list which
types implement which interfaces.

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
