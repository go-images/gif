// Copyright 2026 The go-images authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be found
// in the LICENSE file.

package gif

import (
	"bytes"
	"image"
	"image/color"
	"os"
	"testing"
)

// firstDifference is the first row on which two images differ, or -1.
func firstDifference(a, b image.Image, rows int) int {
	for y := 0; y < rows; y++ {
		for x := a.Bounds().Min.X; x < a.Bounds().Max.X; x++ {
			ar, ag, ab, aa := a.At(x, y).RGBA()
			br, bg, bb, ba := b.At(x, y).RGBA()
			if ar != br || ag != bg || ab != bb || aa != ba {
				return y
			}
		}
	}
	return -1
}

// TestThePartialRowsAreTheRealRowsAndAllOfThem.
//
// ⛔ Two claims, and the second is the one that is easy to forget. A row COUNTED as
// arrived must hold what the complete decode holds. And the count must be the
// LARGEST that is true, because under-claiming is invisible to the first check and
// shows a caller less of the picture than arrived.
//
// Here the second is exact rather than approximate, measured before it was
// asserted: the LZW reader writes into the frame as it goes, so the row where the
// bytes stopped is partly written and differs. Over every fixture here the first
// differing row was the count itself.
func TestThePartialRowsAreTheRealRowsAndAllOfThem(t *testing.T) {
	for _, name := range []string{
		"video-001.gif",
		"video-001.5bpp.gif", // A five-bit literal width, so a different LZW path.
		"video-005.gray.gif",
		"triangle-001.gif",
	} {
		t.Run(name, func(t *testing.T) {
			full, err := os.ReadFile("testdata/" + name)
			if err != nil {
				t.Skipf("no such fixture: %v", err)
			}
			whole, err := Decode(bytes.NewReader(full))
			if err != nil {
				t.Fatalf("the complete fixture does not decode, so nothing below "+
					"means anything: %v", err)
			}
			height := whole.Bounds().Dy()

			seen := 0
			for _, pct := range []int{20, 40, 60, 80, 95} {
				img, rows, err := DecodePartial(bytes.NewReader(full[:len(full)*pct/100]))
				if err == nil {
					t.Fatalf("%d%% of the file decoded completely, so this cut "+
						"exercises nothing", pct)
				}
				if img == nil {
					continue // Too little for a row yet; a larger cut will say more.
				}
				if rows <= 0 {
					t.Errorf("%d%%: an image came back with %d rows", pct, rows)
					continue
				}
				if img.Bounds() != whole.Bounds() {
					t.Errorf("%d%%: bounds %v, want the full size %v",
						pct, img.Bounds(), whole.Bounds())
				}
				fd := firstDifference(img, whole, height)
				switch {
				case fd >= 0 && fd < rows:
					t.Errorf("%d%%: %d rows were offered and row %d already differs "+
						"from the complete decode", pct, rows, fd)
				case fd > rows:
					t.Errorf("%d%%: %d rows offered but the picture is right up to "+
						"row %d — %d rows that arrived are being withheld",
						pct, rows, fd, fd-rows)
				}
				if rows < seen {
					t.Errorf("%d%%: %d rows, fewer than the %d a smaller cut offered",
						pct, rows, seen)
				}
				seen = rows
			}
			if seen == 0 {
				t.Errorf("no cut of this file ever produced a row, so the fixture " +
					"proves nothing")
			}
		})
	}
}

// TestACompleteImageSaysEveryRow: the ordinary case still works, and the count is
// all that distinguishes it from a partial one.
func TestACompleteImageSaysEveryRow(t *testing.T) {
	full, err := os.ReadFile("testdata/video-001.gif")
	if err != nil {
		t.Fatal(err)
	}
	img, rows, err := DecodePartial(bytes.NewReader(full))
	if err != nil {
		t.Fatalf("DecodePartial on a whole file: %v", err)
	}
	if img == nil {
		t.Fatal("no image")
	}
	if rows != img.Bounds().Dy() {
		t.Errorf("rows = %d, want every one of %d", rows, img.Bounds().Dy())
	}
}

// TestAnInterlacedFrameIsRefusedPartially.
//
// ⛔ The rows arrive in passes covering the whole frame — every eighth row, then
// the gaps between them — and uninterlace has not run, so what is in the buffer is
// not "the first n rows" of anything. A caller told forty rows would draw thirty-
// five of them blank.
//
// A nil image AND zero rows, because a caller checks one or the other and only one
// of those mistakes is visible.
func TestAnInterlacedFrameIsRefusedPartially(t *testing.T) {
	full, err := os.ReadFile("testdata/video-001.interlaced.gif")
	if err != nil {
		t.Skipf("no such fixture: %v", err)
	}
	if _, err := Decode(bytes.NewReader(full)); err != nil {
		t.Fatalf("the complete fixture does not decode: %v", err)
	}
	for _, pct := range []int{20, 40, 60, 80} {
		img, rows, err := DecodePartial(bytes.NewReader(full[:len(full)*pct/100]))
		if err == nil {
			t.Fatalf("%d%% decoded completely", pct)
		}
		if img != nil {
			t.Errorf("%d%%: an image was offered for an interlaced frame", pct)
		}
		if rows != 0 {
			t.Errorf("%d%%: rows = %d, want 0", pct, rows)
		}
	}
}

// TestATruncatedAnimationGivesItsFirstFrameWhole.
//
// ⛔ Written first as "a truncated later frame yields the complete first one", with
// a branch in DecodePartial to do it. The branch was UNREACHABLE: the decoder
// returns as soon as one frame is whole when it is not keeping all of them, so a
// cut anywhere after the first frame decodes SUCCESSFULLY. The branch is gone and
// this asserts what happens, which is the better answer anyway — an animation's
// picture is its first frame, and each one after it is a patch over the one before
// with its own disposal rule.
//
// The fixture is written here rather than read, because Go's testdata has no
// animated GIF. It is the frame STRUCTURE being tested and not fidelity, so the
// encoder writing what the decoder reads is not the montage conceding the question.
func TestATruncatedAnimationGivesItsFirstFrameWhole(t *testing.T) {
	pal := color.Palette{color.Black, color.White, color.RGBA{R: 255, A: 255}}
	frame := func(v uint8) *image.Paletted {
		m := image.NewPaletted(image.Rect(0, 0, 64, 64), pal)
		for i := range m.Pix {
			m.Pix[i] = v
		}
		return m
	}
	var buf bytes.Buffer
	if err := EncodeAll(&buf, &GIF{
		Image: []*image.Paletted{frame(1), frame(2), frame(0)},
		Delay: []int{10, 10, 10},
	}); err != nil {
		t.Fatal(err)
	}
	full := buf.Bytes()
	whole, err := Decode(bytes.NewReader(full))
	if err != nil {
		t.Fatalf("the animation does not decode: %v", err)
	}

	// The premise: the file really does hold more than one frame, or a cut after
	// the first proves nothing. DecodeAll keeps them, so it can count.
	all, err := DecodeAll(bytes.NewReader(full))
	if err != nil {
		t.Fatal(err)
	}
	if len(all.Image) < 2 {
		t.Fatalf("the fixture holds %d frame(s); this test needs several", len(all.Image))
	}

	// Find where the first frame ends: the smallest cut that decodes at all.
	first := -1
	for n := 1; n <= len(full); n++ {
		if _, _, err := DecodePartial(bytes.NewReader(full[:n])); err == nil {
			first = n
			break
		}
	}
	if first < 0 || first >= len(full) {
		t.Fatalf("no cut short of the whole file decoded, so nothing is being "+
			"asserted about later frames (first=%d, len=%d)", first, len(full))
	}

	img, rows, err := DecodePartial(bytes.NewReader(full[:first]))
	if err != nil {
		t.Fatalf("a cut holding the whole first frame reported %v", err)
	}
	if rows != img.Bounds().Dy() {
		t.Errorf("rows = %d, want the whole first frame's %d", rows, img.Bounds().Dy())
	}
	if fd := firstDifference(img, whole, rows); fd >= 0 {
		t.Errorf("the first frame differs from the complete decode at row %d", fd)
	}
}

// TestNothingIsOfferedBeforeTheFirstRow: a cut inside the header or the colour
// table has a size and no picture.
func TestNothingIsOfferedBeforeTheFirstRow(t *testing.T) {
	full, err := os.ReadFile("testdata/video-001.gif")
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range []int{0, 6, 13, 40} {
		if n > len(full) {
			break
		}
		img, rows, err := DecodePartial(bytes.NewReader(full[:n]))
		if err == nil {
			t.Errorf("%d bytes decoded completely", n)
		}
		if img != nil || rows != 0 {
			t.Errorf("%d bytes offered an image with %d rows", n, rows)
		}
	}
}
