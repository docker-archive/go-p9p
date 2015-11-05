/*
Package p9pnew implements a compliant 9P2000 client and server library for use
in modern, production Go services. This package differentiates itself in that
is has departed from the plan 9 implementation primitives and better follows
idiomatic Go style.

Multiversion Support

Currently, there is not multiversion support. The hooks and functionality are
in place to add multi-version support. Generally, the correct space to do this
is in the codec. Types, such as Dir, simply need to be extended to support the
possibility of extra fields.
*/
package p9pnew
