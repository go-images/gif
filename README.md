# gif

> **A fork of Go's `image/gif`**, under the same BSD-3-Clause licence.
> The import path is `github.com/go-images/gif`. Every change is stated in
> [NOTICE](NOTICE), and Go's own test suite passes here unaltered in
> substance.

**What differs:** `DecodePartial` says what a stream that stopped early did
decode.

```go
img, rows, err := gif.DecodePartial(r)
```

The first frame at its full declared size, the number of pixel rows that are
complete counting from the top, and the error that stopped the decode — `nil`
when the frame arrived whole. Rows past the count hold whatever the frame was
allocated with, so a caller draws the first `rows` and nothing else.

`Decode` is all-or-nothing, and for a file still arriving that is the same as
having nothing. `image/jpeg` and `image/png` answer the same way, so this is a
gap in the shape of every decoder rather than in one of them.

For a 150×103 picture:

| bytes given | rows offered | |
|---:|---:|---:|
| 20% | 21 | 20.4% |
| 40% | 43 | 41.7% |
| 60% | 61 | 59.2% |
| 80% | 79 | 76.7% |
| 100% | 103 | 100% |

## The change is small because the decoder already did the work

A frame's pixels are read with one `readFull`, which writes what arrived before
reporting that it ran out. **The rows were already in the buffer**; only the
count and the frame were being discarded.

## The count is exact, in both directions

It never names more than arrived, and it never names less. Under-claiming is
invisible to a check that only verifies the rows it offers, and it shows a
caller less of the picture than it has. Over every fixture here the first row
that differs from the complete decode **is** the count — measured before it was
asserted, and the test asserts the equality rather than a tolerance.

## What is refused, and what is not

An **interlaced** frame is refused, with a nil image and zero rows: its rows
arrive in passes covering the whole frame — every eighth row, then the gaps
between them — and `uninterlace` has not run, so what is in the buffer is not
"the first n rows" of anything.

An **animation** is not a special case. Only the first frame is ever reported,
which costs nothing: the decoder returns as soon as one frame is whole, so a
truncated animation whose first frame arrived decodes *successfully* with every
row. Each frame after the first is a patch over the one before with its own
disposal rule, so a later frame's rows are not something a caller could draw
alone.

## Everything else

Identical to the standard library, including `Encode` and `EncodeAll`. Use it
exactly as you would use `image/gif`.
