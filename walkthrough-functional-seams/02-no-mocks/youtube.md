---
title: Tests Without Mocks in Go? (Functional Seams)
tags:
  - golang
  - go
  - testing
  - mocks
  - functional programming
  - software design
  - shorts
visibility: public
---

#Shorts

Most Go test suites are 50% mock setup. There's a way to delete all of it.

In this short, I show how function parameters replace mock libraries entirely. The trick: build a small library of named behavior stubs (`ChargeOK`, `ChargeDeclined`, `NotifyOK`, etc.), then compose them in tests. Each test becomes a one-line sentence — "compose ChargeOK with NotifyOK, expect no error."

When you need to capture call arguments, a closure over a local slice does the job. No mock framework. No `expect().to.have.been.called.with()` DSL.

I haven't written a mock in two years. This is why.

→ Part 1 (intro to functional seams): [add link]
→ More Go content on this channel

What's your favorite testing pattern in Go? Drop a comment.

#golang #testing #softwaredesign #functionalprogramming #programming
