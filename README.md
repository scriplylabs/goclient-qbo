# quickbooks-go (Idiomatic Go Fork)

An idiomatic Go client library for the QuickBooks Online API.

This project is a public fork of the original:
https://github.com/rwestlund/quickbooks-go

It introduces API and structural changes to better align with Go conventions and modern Go practices.

> ⚠️ **This is NOT a drop-in replacement for the original project.**
> The public API has changed.

---

## Why This Fork Exists

The original project provides a solid QuickBooks client implementation.  
This fork was created to:

- Improve API ergonomics (pointers for optional fields)
- Align naming and structure with idiomatic Go
- Modernize dependencies
- Add more APIs (class, and reports)

If you depend on the original API surface, you should continue using the upstream project.

---

## Installation

go get github.com/scriplylabs/goclient-qbo
