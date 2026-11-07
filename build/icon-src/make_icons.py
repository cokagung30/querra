#!/usr/bin/env python3
"""Generates Querra's icon assets from the master logo.

Requires Pillow. Run from the repository root, then let Wails turn the PNGs
into platform formats:

    python3 build/icon-src/make_icons.py
    wails3 task common:generate:icons
    wails3 generate icons -input build/darwin/dmg-file-icon.png \
        -macfilename build/darwin/dmg-file-icon.icns -windowsfilename ""

Reads build/icon-src/logo-original.png (transparent PNG) and writes:

    build/icon-src/logo.png              cleaned logo, 1024×1024, transparent (Windows .ico)
    build/appicon.png                    logo on a macOS squircle (.icns for macOS 12–15)
    build/appicon.icon/Assets/logo.png   foreground layer of the macOS 26 Icon Composer icon
    build/darwin/dmg-file-icon.png       icon of the .dmg file itself
    build/darwin/dmg-background.png      DMG window background
"""

from __future__ import annotations

from pathlib import Path

from PIL import Image, ImageChops, ImageDraw, ImageFilter, ImageFont

HERE = Path(__file__).resolve().parent
BUILD = HERE.parent
SIZE = 1024

# Squircle background, top to bottom: deep navy so the blue-purple logo stands
# out. Keep in sync with the "fill" colour in build/appicon.icon/icon.json.
BG_TOP = (32, 38, 82)
BG_BOTTOM = (10, 12, 34)

# DMG window background: dark at the top left, fading to near white.
DMG_SIZE = (540, 380)
DMG_DARK = (16, 20, 48)
DMG_LIGHT = (244, 246, 252)
DMG_TEXT = (20, 24, 51)
FONTS = ["/System/Library/Fonts/SFNS.ttf", "/System/Library/Fonts/Helvetica.ttc"]


def clean(logo: Image.Image) -> Image.Image:
    """Removes the stray semi-transparent specks left by background removal.

    Keeps only pixels within 4 px of the solid (alpha > 128) shapes, which
    preserves anti-aliased edges, then crops to the content.
    """
    alpha = logo.getchannel("A")
    solid = alpha.point(lambda v: 255 if v > 128 else 0)
    near_solid = solid.filter(ImageFilter.MaxFilter(9))
    cleaned = logo.copy()
    cleaned.putalpha(ImageChops.multiply(alpha, near_solid))
    return cleaned.crop(cleaned.getbbox())


def fit(img: Image.Image, size: int, ratio: float) -> Image.Image:
    """Centres img on a transparent size×size canvas, its longer side ratio·size."""
    scale = size * ratio / max(img.size)
    resized = img.resize((round(img.width * scale), round(img.height * scale)), Image.LANCZOS)
    canvas = Image.new("RGBA", (size, size), (0, 0, 0, 0))
    canvas.alpha_composite(resized, ((size - resized.width) // 2, (size - resized.height) // 2))
    return canvas


def rounded_mask(size: int, box: tuple[int, int, int, int], radius: int, ss: int = 4) -> Image.Image:
    """Anti-aliased rounded-rectangle mask, drawn supersampled."""
    big = Image.new("L", (size * ss, size * ss), 0)
    ImageDraw.Draw(big).rounded_rectangle([c * ss for c in box], radius=radius * ss, fill=255)
    return big.resize((size, size), Image.LANCZOS)


def lerp(a: tuple[int, ...], b: tuple[int, ...], t: float) -> tuple[int, ...]:
    return tuple(round(x + (y - x) * t) for x, y in zip(a, b))


def mac_icon(logo: Image.Image) -> Image.Image:
    """Logo on a gradient squircle with a soft shadow, on Apple's icon grid
    (an 824 px shape inside the 1024 px canvas)."""
    mask = rounded_mask(SIZE, (100, 100, 924, 924), 185)

    shadow_alpha = ImageChops.offset(mask, 0, 12).filter(ImageFilter.GaussianBlur(14))
    shadow = Image.new("RGBA", (SIZE, SIZE), (0, 0, 0, 0))
    shadow.putalpha(shadow_alpha.point(lambda v: int(v * 0.35)))

    gradient = Image.new("RGBA", (1, SIZE))
    for y in range(SIZE):
        gradient.putpixel((0, y), (*lerp(BG_TOP, BG_BOTTOM, y / (SIZE - 1)), 255))
    background = gradient.resize((SIZE, SIZE))
    background.putalpha(mask)

    icon = Image.new("RGBA", (SIZE, SIZE), (0, 0, 0, 0))
    icon.alpha_composite(shadow)
    icon.alpha_composite(background)
    icon.alpha_composite(fit(logo, SIZE, 0.58))
    return icon


def load_font(size: int) -> ImageFont.ImageFont:
    for path in FONTS:
        if Path(path).exists():
            return ImageFont.truetype(path, size)
    return ImageFont.load_default()


def dmg_background(logo: Image.Image) -> Image.Image:
    """Diagonal gradient with the logo and a spaced-out wordmark at the bottom
    right, where the Wails template had its branding."""
    w, h = DMG_SIZE
    bg = Image.new("RGBA", DMG_SIZE)
    pixels = bg.load()
    for y in range(h):
        for x in range(w):
            t = min(1.0, (x / w + y / h) / 1.4)
            pixels[x, y] = (*lerp(DMG_DARK, DMG_LIGHT, t), 255)

    mark = fit(logo, 72, 1.0)
    center_x = 474
    bg.alpha_composite(mark, (center_x - 36, 248))

    draw = ImageDraw.Draw(bg)
    font = load_font(13)
    letters = "QUERRA"
    spacing = 5
    widths = [draw.textlength(ch, font=font) for ch in letters]
    x = center_x - (sum(widths) + spacing * (len(letters) - 1)) / 2
    for ch, cw in zip(letters, widths):
        draw.text((x, 326), ch, font=font, fill=DMG_TEXT)
        x += cw + spacing
    return bg


def main() -> None:
    logo = clean(Image.open(HERE / "logo-original.png").convert("RGBA"))

    fit(logo, SIZE, 0.96).save(HERE / "logo.png")
    icon = mac_icon(logo)
    icon.save(BUILD / "appicon.png")
    composer_assets = BUILD / "appicon.icon" / "Assets"
    composer_assets.mkdir(parents=True, exist_ok=True)
    fit(logo, SIZE, 0.96).save(composer_assets / "logo.png")
    icon.save(BUILD / "darwin" / "dmg-file-icon.png")
    dmg_background(logo).save(BUILD / "darwin" / "dmg-background.png")
    print("icon assets written; now run `wails3 task common:generate:icons`")


if __name__ == "__main__":
    main()
