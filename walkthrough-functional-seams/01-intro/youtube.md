---
title: A Better Way Than Interfaces in Go? (Functional Seams)
tags:
  - golang
  - go
  - functional programming
  - software design
  - testing
  - shorts
visibility: public
---

#Shorts

Most Go developers reach for interfaces when they need a seam in their code. There's a simpler way — a function parameter.

In this short, I show how a single `func(T) U` parameter replaces a typical interface + mock + test setup combo. The result: less code, no mocking framework, and the test reads like a fact.

This is the pattern I now use for ~80% of dependency injection in my own Go projects. Interfaces are still right when multiple methods truly belong together. But for a single behavior — pass a function.

→ Part 2 (tests without mocks): coming soon
→ More Go content on this channel

What patterns are you using for testable Go code? Drop a comment.

#golang #functionalprogramming #softwaredesign #programming #coding
