# -*- coding: utf-8 -*-
"""Génère WasmRedis-soutenance.pptx : diapositives + captures de code annotées.

Les extraits de code sont lus directement dans les fichiers du dépôt, par
numéro de ligne : la présentation ne peut pas diverger du code réel.
"""

import math
import os
import re
import subprocess

from PIL import Image, ImageDraw, ImageFont
from pygments import lex
from pygments.lexers import GoLexer, JsonLexer, JavascriptLexer
from pygments.token import Token

from pptx import Presentation
from pptx.dml.color import RGBColor
from pptx.enum.shapes import MSO_SHAPE
from pptx.enum.text import MSO_ANCHOR, PP_ALIGN
from pptx.oxml.ns import qn
from pptx.util import Emu, Inches, Pt

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
IMG = os.path.join(ROOT, "presentation", "img")
OUT = os.path.join(ROOT, "presentation", "WasmRedis-soutenance.pptx")
os.makedirs(IMG, exist_ok=True)

# ---------------------------------------------------------------- palette ---
INK = "#1a1f1e"
MUTED = "#6f7573"
FAINT = "#9aa0a0"
LINE = "#e6e9e8"
HOVER = "#f5f6f6"
ACCENT = "#217d65"
ACCENT_DARK = "#196750"
ACCENT_SOFT = "#e8f1ed"
ALERT = "#a04a44"
ALERT_SOFT = "#f7ecea"
PENDING = "#d5a449"
RUNNING = "#649ad8"
COMPLETED = "#42ad8b"
FAILED = "#dc8080"

CODE_BG = "#fbfcfb"
CODE_BORDER = "#e2e6e4"
CODE_NUM = "#b9c0be"
HL_BG = "#e9f2ee"

SANS = "Arial"

def rgb(h):
    h = h.lstrip("#")
    return RGBColor(int(h[0:2], 16), int(h[2:4], 16), int(h[4:6], 16))

# ------------------------------------------------------- rendu des extraits ---
SCALE = 2
F_REG = "/usr/share/fonts/TTF/JetBrainsMonoNerdFontMono-Regular.ttf"
F_BOLD = "/usr/share/fonts/TTF/JetBrainsMonoNerdFontMono-Bold.ttf"
F_ITAL = "/usr/share/fonts/TTF/JetBrainsMonoNerdFontMono-Italic.ttf"

TOKEN_STYLES = [
    (Token.Comment, ("#8a9290", False, True)),
    (Token.Keyword.Type, ("#1f6f8b", False, False)),
    (Token.Keyword, ("#9a3b6e", True, False)),
    (Token.Name.Function, ("#2a5db0", True, False)),
    (Token.Name.Builtin, ("#1f6f8b", False, False)),
    (Token.Name.Class, ("#1f6f8b", False, False)),
    (Token.Name.Tag, ("#2a5db0", False, False)),
    (Token.Name.Attribute, ("#1f6f8b", False, False)),
    (Token.String, ("#217d65", False, False)),
    (Token.Number, ("#a6572e", False, False)),
    (Token.Literal, ("#a6572e", False, False)),
    (Token.Operator, ("#7a817f", False, False)),
    (Token.Punctuation, ("#7a817f", False, False)),
    (Token.Name, (INK, False, False)),
    (Token.Text, (INK, False, False)),
]

def style_for(tok):
    node = tok
    while node is not None:
        for key, style in TOKEN_STYLES:
            if node is key:
                return style
        node = node.parent
    return (INK, False, False)

_cache = {}

def tokenize(path, lexer):
    if path in _cache:
        return _cache[path]
    with open(os.path.join(ROOT, path), encoding="utf-8") as handle:
        source = handle.read().replace("\t", "    ")
    lines = [[]]
    for tok, text in lex(source, lexer):
        for i, part in enumerate(text.split("\n")):
            if i:
                lines.append([])
            if part:
                lines[-1].append((style_for(tok), part))
    _cache[path] = lines
    return lines

def lexer_for(path):
    if path.endswith(".go"):
        return GoLexer()
    if path.endswith((".jsx", ".js")):
        return JavascriptLexer()
    return JsonLexer()

def trim(segs):
    kept = list(segs)
    while kept:
        style, text = kept[-1]
        stripped = text.rstrip()
        if stripped:
            kept[-1] = (style, stripped)
            break
        kept.pop()
    return kept or [((INK, False, False), "")]


def file_rows(path, ranges):
    lines = tokenize(path, lexer_for(path))
    rows = []
    for i, (start, end) in enumerate(ranges):
        if i:
            rows.append({"elide": True})
        for number in range(start, end + 1):
            rows.append({"num": number, "segs": trim(lines[number - 1])})
    return rows

def raw_rows(text, lexer, numbers=False):
    holder = [[]]
    for tok, part in lex(text, lexer):
        for i, piece in enumerate(part.split("\n")):
            if i:
                holder.append([])
            if piece:
                holder[-1].append((style_for(tok), piece))
    rows = []
    for i, segs in enumerate(holder):
        if i == len(holder) - 1 and not segs:
            continue
        rows.append({"num": i + 1 if numbers else None, "segs": segs or [((INK, False, False), "")]})
    return rows

def plain_rows(lines):
    palette = {
        "cmd": (INK, True, False),
        "in": (ACCENT, True, False),
        "out": (INK, False, False),
        "dim": (FAINT, False, False),
        "note": (MUTED, False, True),
        "bad": (ALERT, False, False),
    }
    rows = []
    for text, kind in lines:
        rows.append({"num": None, "segs": [(palette[kind], text)] if text else [((INK, False, False), "")]})
    return rows

def wrap(segs, max_cols, indent):
    out = [[]]
    col = 0
    indent = min(indent + 2, max(4, max_cols - 20))
    for style, text in segs:
        i = 0
        while i < len(text):
            room = max_cols - col
            if room <= 0:
                out.append([((FAINT, False, False), " " * indent)])
                col = indent
                room = max_cols - col
            chunk = text[i:i + room]
            out[-1].append((style, chunk))
            col += len(chunk)
            i += len(chunk)
        if not text:
            continue
    return out or [[]]

def render_block(name, rows, badges=None, highlight=(), font_px=15, max_cols=86,
                 show_numbers=True, background=CODE_BG, border=CODE_BORDER):
    badges = badges or {}
    highlight = set(highlight)
    basic = ImageFont.Layout.BASIC
    reg = ImageFont.truetype(F_REG, font_px * SCALE, layout_engine=basic)
    bold = ImageFont.truetype(F_BOLD, font_px * SCALE, layout_engine=basic)
    ital = ImageFont.truetype(F_ITAL, font_px * SCALE, layout_engine=basic)
    char_w = reg.getlength("M") / SCALE
    line_h = round(font_px * 1.62)

    laid = []
    for row in rows:
        if row.get("elide"):
            laid.append({"elide": True, "num": None, "badge": None})
            continue
        indent = len(row["segs"][0][1]) - len(row["segs"][0][1].lstrip()) if row["segs"] else 0
        pieces = wrap(row["segs"], max_cols, indent)
        for i, piece in enumerate(pieces):
            laid.append({
                "segs": piece,
                "num": row["num"] if i == 0 else None,
                "cont": i > 0,
                "badge": badges.get(row["num"]) if i == 0 else None,
                "hl": row["num"] in highlight,
            })

    pad_x, pad_y = 15, 13
    badge_w = 24 if badges else 0
    width_digits = max([len(str(r["num"])) for r in laid if r.get("num")] or [1])
    gutter = (width_digits * char_w + 14) if show_numbers else 0
    code_x = pad_x + badge_w + gutter
    longest = max((sum(len(t) for _, t in r.get("segs", [])) for r in laid), default=10)
    width = int(code_x + longest * char_w + pad_x + 4)
    height = int(pad_y * 2 + len(laid) * line_h)

    image = Image.new("RGB", (width * SCALE, height * SCALE), "#ffffff")
    draw = ImageDraw.Draw(image)
    draw.rounded_rectangle([0, 0, width * SCALE - 1, height * SCALE - 1], radius=9 * SCALE,
                           fill=background, outline=border, width=max(1, SCALE))

    for i, row in enumerate(laid):
        top = pad_y + i * line_h
        centre = top + line_h / 2
        if row.get("hl"):
            draw.rectangle([(pad_x + badge_w - 6) * SCALE, top * SCALE,
                            (width - pad_x + 2) * SCALE, (top + line_h) * SCALE], fill=HL_BG)
            draw.rectangle([(pad_x + badge_w - 6) * SCALE, top * SCALE,
                            (pad_x + badge_w - 6 + 2.5) * SCALE, (top + line_h) * SCALE], fill=ACCENT)
        if row.get("elide"):
            draw.text((code_x * SCALE, centre * SCALE), "⋮", font=reg, fill=CODE_NUM, anchor="lm")
            continue
        if row.get("badge"):
            r = 8.6
            cx = pad_x + badge_w / 2 - 2
            draw.ellipse([(cx - r) * SCALE, (centre - r) * SCALE, (cx + r) * SCALE, (centre + r) * SCALE],
                         fill=ACCENT)
            small = ImageFont.truetype(F_BOLD, int(font_px * 0.76 * SCALE), layout_engine=basic)
            draw.text((cx * SCALE, (centre + 0.3) * SCALE), str(row["badge"]), font=small,
                      fill="#ffffff", anchor="mm")
        if show_numbers and row.get("num"):
            draw.text(((code_x - 14) * SCALE, centre * SCALE), str(row["num"]), font=reg,
                      fill=CODE_NUM, anchor="rm")
        if row.get("cont"):
            draw.text(((code_x - 14) * SCALE, centre * SCALE), "↳", font=reg, fill="#d2d7d5", anchor="rm")
        x = code_x
        for (colour, is_bold, is_ital), text in row["segs"]:
            font = bold if is_bold else (ital if is_ital else reg)
            draw.text((x * SCALE, centre * SCALE), text, font=font, fill=colour, anchor="lm")
            x += len(text) * char_w

    path = os.path.join(IMG, name + ".png")
    image.save(path)
    return path

def code_png(name, path, ranges, **kwargs):
    return render_block(name, file_rows(path, ranges), **kwargs)

def json_png(name, text, **kwargs):
    return render_block(name, raw_rows(text, JsonLexer()), show_numbers=False, **kwargs)

def term_png(name, lines, **kwargs):
    return render_block(name, plain_rows(lines), show_numbers=False,
                        background="#f7f8f8", border="#e2e6e4", **kwargs)


# ------------------------------------------------------ mise en page pptx ---
W, H = 13.333, 7.5
ML = 0.68
CW = W - 2 * ML
TOP = 1.70
BOTTOM = 6.84
CH = BOTTOM - TOP

prs = Presentation()
prs.slide_width = Inches(W)
prs.slide_height = Inches(H)
BLANK = prs.slide_layouts[6]
MARKUP = re.compile(r"(\*[^*]+\*|`[^`]+`)")

def runs(p, text, size, colour, bold=False, italic=False, font=SANS):
    for part in MARKUP.split(text):
        if not part:
            continue
        run = p.add_run()
        strong = part.startswith("*") and part.endswith("*") and len(part) > 2
        mono = part.startswith("`") and part.endswith("`") and len(part) > 2
        run.text = part[1:-1] if (strong or mono) else part
        f = run.font
        f.name = "Consolas" if mono else font
        f.size = Pt(size * 0.94 if mono else size)
        f.bold = bold or strong
        f.italic = italic
        if mono:
            f.color.rgb = rgb(ACCENT_DARK)
        elif strong and colour in (MUTED, FAINT):
            f.color.rgb = rgb(INK)
        else:
            f.color.rgb = rgb(colour)

def textbox(slide, x, y, w, h):
    box = slide.shapes.add_textbox(Inches(x), Inches(y), Inches(w), Inches(h))
    tf = box.text_frame
    tf.word_wrap = True
    tf.margin_left = tf.margin_right = tf.margin_top = tf.margin_bottom = 0
    return tf

def para(tf, text, size=13, colour=INK, bold=False, italic=False, first=False,
         before=0, after=0, spacing=1.2, align=PP_ALIGN.LEFT, font=SANS):
    p = tf.paragraphs[0] if first else tf.add_paragraph()
    p.alignment = align
    p.line_spacing = spacing
    p.space_before = Pt(before)
    p.space_after = Pt(after)
    runs(p, text, size, colour, bold, italic, font)
    return p

def est_h(text, w, size, spacing=1.2):
    clean = re.sub(r"[*`]", "", text)
    per_line = max(12, 72 * w / (0.512 * size))
    return max(1, math.ceil(len(clean) / per_line)) * size * spacing / 72

def bare(shape):
    style = shape._element.find(qn("p:style"))
    if style is not None:
        shape._element.remove(style)
    shape.shadow.inherit = False
    return shape


def rect(slide, x, y, w, h, fill=None, line=None, width=0.75, shape=MSO_SHAPE.RECTANGLE):
    s = bare(slide.shapes.add_shape(shape, Inches(x), Inches(y), Inches(w), Inches(h)))
    if fill:
        s.fill.solid()
        s.fill.fore_color.rgb = rgb(fill)
    else:
        s.fill.background()
    if line:
        s.line.color.rgb = rgb(line)
        s.line.width = Pt(width)
    else:
        s.line.fill.background()
    s.shadow.inherit = False
    s.text_frame.word_wrap = True
    s.text_frame.margin_left = s.text_frame.margin_right = Inches(0.1)
    s.text_frame.margin_top = s.text_frame.margin_bottom = Inches(0.05)
    return s

def hline(slide, x, y, w, colour=LINE, width=0.75):
    s = bare(slide.shapes.add_shape(MSO_SHAPE.RECTANGLE, Inches(x), Inches(y), Inches(w), Emu(1)))
    s.fill.solid()
    s.fill.fore_color.rgb = rgb(colour)
    s.line.fill.background()
    s.shadow.inherit = False
    return s

_slides = []

def slide(kicker=None, title=None, lead=None):
    s = prs.slides.add_slide(BLANK)
    _slides.append(s)
    if kicker:
        tf = textbox(s, ML, 0.40, CW, 0.22)
        para(tf, kicker.upper(), size=10, colour=ACCENT, bold=True, first=True)
    if title:
        tf = textbox(s, ML, 0.63, CW, 0.55)
        para(tf, title, size=26, colour=INK, bold=True, first=True, spacing=1.0)
    if lead:
        tf = textbox(s, ML, 1.24, CW, 0.30)
        para(tf, lead, size=13, colour=MUTED, first=True)
        hline(s, ML, 1.62, CW)
    elif title:
        hline(s, ML, 1.36, CW)
    return s

def divider(number, title, lines):
    s = prs.slides.add_slide(BLANK)
    _slides.append(s)
    rect(s, 0, 0, W, H, fill=ACCENT)
    tf = textbox(s, 1.5, 2.55, 10.3, 0.4)
    para(tf, "PARTIE %s" % number, size=12, colour="#a9cfc2", bold=True, first=True)
    tf = textbox(s, 1.5, 2.98, 10.3, 1.0)
    para(tf, title, size=34, colour="#ffffff", bold=True, first=True, spacing=1.05)
    tf = textbox(s, 1.5, 4.28, 9.4, 1.0)
    for i, text in enumerate(lines):
        para(tf, text, size=13.5, colour="#cde3da", first=(i == 0), after=4)
    return s

def annotations(slide, x, y, w, items, size=12.5, gap=0.19, start=1):
    top = y
    for i, text in enumerate(items):
        n = start + i
        badge = rect(slide, x, top + 0.015, 0.235, 0.235, fill=ACCENT, shape=MSO_SHAPE.OVAL)
        p = badge.text_frame.paragraphs[0]
        p.alignment = PP_ALIGN.CENTER
        badge.text_frame.margin_left = badge.text_frame.margin_right = 0
        badge.text_frame.margin_top = badge.text_frame.margin_bottom = 0
        badge.text_frame.vertical_anchor = MSO_ANCHOR.MIDDLE
        runs(p, str(n), 9, "#ffffff", bold=True)
        tw = w - 0.36
        tf = textbox(slide, x + 0.36, top - 0.02, tw, 0.3)
        para(tf, text, size=size, colour=MUTED, first=True, spacing=1.18)
        top += max(0.27, est_h(text, tw, size, 1.18)) + gap
    return top

def bullets(slide, x, y, w, items, size=13, gap=0.15, marker=True, colour=MUTED):
    top = y
    for text in items:
        if marker:
            rect(slide, x + 0.03, top + 0.085, 0.075, 0.075, fill=ACCENT, shape=MSO_SHAPE.OVAL)
        tw = w - (0.28 if marker else 0)
        tf = textbox(slide, x + (0.28 if marker else 0), y=top - 0.02, w=tw, h=0.3)
        para(tf, text, size=size, colour=colour, first=True, spacing=1.2)
        top += max(0.25, est_h(text, tw, size, 1.2)) + gap
    return top

def card(slide, x, y, w, h, title, body, tint=None, accent=ACCENT, title_size=13, body_size=11.5):
    rect(slide, x, y, w, h, fill=tint or "#ffffff", line=LINE)
    rect(slide, x, y, 0.035, h, fill=accent)
    tf = textbox(slide, x + 0.24, y + 0.2, w - 0.44, 0.3)
    para(tf, title, size=title_size, colour=INK, bold=True, first=True)
    if body:
        tf = textbox(slide, x + 0.24, y + 0.2 + title_size * 1.35 / 72 + 0.06, w - 0.44, h - 0.6)
        for i, text in enumerate(body):
            para(tf, text, size=body_size, colour=MUTED, first=(i == 0), after=3, spacing=1.16)

def kpi(slide, x, y, w, value, label, colour=ACCENT):
    tf = textbox(slide, x, y, w, 0.5)
    para(tf, value, size=27, colour=colour, bold=True, first=True, spacing=1.0)
    tf = textbox(slide, x, y + 0.48, w, 0.5)
    para(tf, label, size=10.5, colour=FAINT, first=True, spacing=1.15)

def table(slide, x, y, w, headers, rows, widths, size=11.5, head_size=9.5, row_h=None):
    total = sum(widths)
    cols = [w * v / total for v in widths]
    cx = x
    for i, head in enumerate(headers):
        tf = textbox(slide, cx, y, cols[i] - 0.14, 0.24)
        para(tf, head.upper(), size=head_size, colour=FAINT, bold=True, first=True)
        cx += cols[i]
    top = y + 0.28
    hline(slide, x, top, w, LINE)
    top += 0.10
    for row in rows:
        height = row_h or max(0.28, max(est_h(str(c), cols[i] - 0.16, size, 1.18)
                                        for i, c in enumerate(row)))
        cx = x
        for i, cell in enumerate(row):
            tf = textbox(slide, cx, top, cols[i] - 0.14, height)
            para(tf, str(cell), size=size, colour=INK if i == 0 else MUTED, first=True, spacing=1.18)
            cx += cols[i]
        top += height + 0.13
        hline(slide, x, top - 0.065, w, "#f0f2f1")
    return top

def place(slide, path, x, y, max_w, max_h, align="left", frame=False):
    w_px, h_px = Image.open(path).size
    ratio = w_px / h_px
    w = max_w
    h = w / ratio
    if h > max_h:
        h = max_h
        w = h * ratio
    if align == "center":
        x = x + (max_w - w) / 2
    slide.shapes.add_picture(path, Inches(x), Inches(y), Inches(w), Inches(h))
    if frame:
        rect(slide, x, y, w, h, fill=None, line=LINE)
    return (x, y, w, h)

def crop(name, source, top=0.0, bottom=1.0, left=0.0, right=1.0):
    im = Image.open(os.path.join(IMG, source + ".png"))
    w, h = im.size
    out = os.path.join(IMG, name + ".png")
    im.crop((int(w * left), int(h * top), int(w * right), int(h * bottom))).save(out)
    return out


def flow(slide, x, y, w, steps, size=11, box_h=0.62, gap=0.16):
    n = len(steps)
    bw = (w - gap * (n - 1)) / n
    for i, (label, sub) in enumerate(steps):
        bx = x + i * (bw + gap)
        rect(slide, bx, y, bw, box_h, fill=ACCENT_SOFT if i % 2 == 0 else "#ffffff", line=LINE)
        tf = textbox(slide, bx + 0.12, y + 0.10, bw - 0.24, box_h - 0.2)
        para(tf, label, size=size, colour=INK, bold=True, first=True, spacing=1.1)
        if sub:
            para(tf, sub, size=size - 1.5, colour=MUTED, spacing=1.1)
        if i < n - 1:
            tri = bare(slide.shapes.add_shape(MSO_SHAPE.ISOSCELES_TRIANGLE,
                                              Inches(bx + bw + gap / 2 - 0.045),
                                              Inches(y + box_h / 2 - 0.05),
                                              Inches(0.09), Inches(0.10)))
            tri.rotation = 90
            tri.fill.solid()
            tri.fill.fore_color.rgb = rgb(FAINT)
            tri.line.fill.background()
            tri.shadow.inherit = False

def note_h(text, size=11.5):
    return max(0.46, est_h(text, CW - 0.44, size, 1.22) + 0.26)


def note(slide, text, y=None, tint=ACCENT_SOFT, colour=ACCENT_DARK, size=11.5):
    h = note_h(text, size)
    y = BOTTOM - h if y is None else y
    rect(slide, ML, y, CW, h, fill=tint, line=None)
    tf = textbox(slide, ML + 0.22, y + 0.13, CW - 0.44, h - 0.2)
    para(tf, text, size=size, colour=colour, first=True, spacing=1.22)
    return h

def code_slide(kicker, title, img, items, lead=None, footer=None, size=12.5,
               max_w=7.5, ann_w=4.35, start=1):
    s = slide(kicker, title, lead)
    top = TOP + (0.26 if lead else 0)
    room = (BOTTOM - top) - ((note_h(footer) + 0.26) if footer else 0.04)
    x, y, w, h = place(s, img, ML, top, min(max_w, CW - 0.45 - ann_w), room)
    ax = ML + w + 0.45
    annotations(s, ax, top + 0.02, W - ML - ax, items, size=size, start=start)
    if footer:
        note(s, footer)
    return s


def arrow_down(slide, x, y, h=0.26, colour="#c8cfcd"):
    s = bare(slide.shapes.add_shape(MSO_SHAPE.DOWN_ARROW, Inches(x - 0.055), Inches(y),
                                    Inches(0.11), Inches(h)))
    s.fill.solid()
    s.fill.fore_color.rgb = rgb(colour)
    s.line.fill.background()
    return s


def finish():
    for i, s in enumerate(_slides):
        if i == 0:
            continue
        tf = textbox(s, ML, 6.97, 6.0, 0.24)
        para(tf, "WasmRedis · moteur clé-valeur en Go", size=9, colour="#c2c7c5", first=True)
        tf = textbox(s, W - ML - 1.2, 6.97, 1.2, 0.24)
        para(tf, "%02d" % (i + 1), size=9, colour="#c2c7c5", first=True, align=PP_ALIGN.RIGHT)
    prs.save(OUT)
    return OUT


# =============================================================== 1. LE PROJET
s = slide()
rect(s, 0, 0, 0.14, H, fill=ACCENT)
tf = textbox(s, 1.45, 2.08, 10.4, 0.3)
para(tf, "PROJET GO · SOUTENANCE", size=11.5, colour=ACCENT, bold=True, first=True)
tf = textbox(s, 1.42, 2.42, 10.4, 1.1)
para(tf, "WasmRedis", size=54, colour=INK, bold=True, first=True, spacing=1.0)
tf = textbox(s, 1.45, 3.62, 9.6, 0.8)
para(tf, "Un moteur clé-valeur persistant écrit en Go,\net son tableau de bord React",
     size=17, colour=MUTED, first=True, spacing=1.35)
hline(s, 1.45, 4.68, 3.4, ACCENT)
tf = textbox(s, 1.45, 4.92, 10.0, 0.3)
para(tf, "Parser  ·  Journal AOF  ·  Snapshots  ·  Index typés  ·  TTL  ·  API HTTP",
     size=12, colour=FAINT, first=True)
rect(s, 1.45, 5.52, 8.6, 0.62, fill=ACCENT_SOFT)
tf = textbox(s, 1.66, 5.71, 8.2, 0.3)
para(tf, "Fil rouge : que se passe-t-il, précisément, quand on tape  `SET age 42`  ?",
     size=13, colour=ACCENT_DARK, bold=True, first=True)

# ---------------------------------------------------------------------------
s = slide("Vue d'ensemble", "Le projet en trois phrases")
top = TOP
for text in [
    "WasmRedis est un *magasin clé-valeur* : il garde des paires clé → valeur en mémoire vive, "
    "et les écrit sur le disque pour qu'elles survivent à un redémarrage.",
    "On lui parle en *commandes* — `SET`, `GET`, `DEL`, `PING`, `GET WHERE` — depuis un terminal "
    "interactif ou une API HTTP.",
    "Un *tableau de bord React* se branche sur cette API pour piloter des files de tâches et "
    "montrer le moteur à l'œuvre.",
]:
    tf = textbox(s, ML, top, CW - 0.4, 0.5)
    para(tf, text, size=15, colour=MUTED, first=True, spacing=1.32)
    top += est_h(text, CW - 0.4, 15, 1.32) + 0.30

hline(s, ML, 4.32, CW)
for i, (value, label) in enumerate([
    ("1 749", "lignes de Go\nhors tests"),
    ("7", "paquets, sans\ndépendance circulaire"),
    ("2", "dépendances externes\nbtree et flock"),
    ("28 + 4", "tests Go\net tests navigateur"),
    ("0", "commentaire dans\nle code source"),
]):
    kpi(s, ML + i * (CW / 5), 4.62, CW / 5 - 0.3, value, label)
note(s, "Le code ne porte aucun commentaire : c'est une règle du projet. Les explications vivent "
        "dans le README et dans cette présentation.", y=6.28)

# ---------------------------------------------------------------------------
s = slide("Le périmètre", "Deux moitiés qu'il ne faut pas confondre")
card(s, ML, TOP, CW / 2 - 0.22, 2.55,
     "Le magasin clé-valeur — persisté",
     ["Chaque `SET` et chaque `DEL` finit dans `data/aof.log`, "
      "puis dans `data/snapshot.json`.",
      "Arrêtez le programme, relancez-le : les clés sont toujours là.",
      "C'est l'objet des cinq phases de la feuille de route."],
     accent=ACCENT, body_size=12.5)
card(s, ML + CW / 2 + 0.22, TOP, CW / 2 - 0.22, 2.55,
     "Les files de tâches — en mémoire",
     ["Elles vivent dans une `map` du serveur HTTP, et rien d'autre.",
      "Redémarrez le serveur : les files ont disparu.",
      "Elles servent à démontrer l'API et l'interface, pas à stocker durablement."],
     accent=PENDING, body_size=12.5)
tf = textbox(s, ML, TOP + 2.90, CW, 0.9)
para(tf, "Pourquoi cette dissymétrie ?", size=14, colour=INK, bold=True, first=True, after=7)
para(tf, "La persistance demandée portait sur le stockage clé-valeur. Persister aussi les files "
         "aurait doublé le travail sans rien démontrer de plus : c'est le même journal, le même "
         "snapshot, la même reprise. Le choix est assumé et écrit noir sur blanc dans l'interface "
         "elle-même, en bas de l'écran.", size=13, colour=MUTED, spacing=1.3)
note(s, "« Files en mémoire : elles disparaîssent au redémarrage du serveur. » "
        "— le texte affiché sous la liste des tâches.", y=6.10)

# ---------------------------------------------------------------------------
s = slide("Architecture", "Sept paquets, et des dépendances qui ne remontent jamais")

def node(x, y, w, h, title, sub, tint="#ffffff", accent=LINE, bold_title=True):
    rect(s, x, y, w, h, fill=tint, line=LINE)
    tf = textbox(s, x + 0.16, y + 0.13, w - 0.32, 0.28)
    para(tf, title, size=12, colour=INK, bold=bold_title, first=True, font="Consolas")
    tf = textbox(s, x + 0.16, y + 0.40, w - 0.32, 0.4)
    para(tf, sub, size=10, colour=MUTED, first=True, spacing=1.15)

row_w = (CW - 3 * 0.22) / 4
node(ML, 1.80, row_w, 0.74, "cli.go", "terminal interactif")
node(ML + row_w + 0.22, 1.80, row_w, 0.74, "server.go", "serveur HTTP + démo")
node(ML + 2 * (row_w + 0.22), 1.80, row_w, 0.74, "web/src", "React + Tailwind")
node(ML + 3 * (row_w + 0.22), 1.80, row_w, 0.74, "main.go", "réglages, signaux, arrêt")
node(ML + row_w + 0.22, 2.86, row_w * 2 + 0.22, 0.74, "internal/api", "routes HTTP, JSON, codes d'erreur")
node(ML, 3.92, CW, 0.86, "internal/engine", "état, commandes, persistance, requêtes, expiration, files de tâches",
     tint=ACCENT_SOFT)
low_w = (CW - 3 * 0.22) / 4
for i, (name, sub) in enumerate([
    ("internal/command", "parser, types de commandes"),
    ("internal/index", "index inversé + B-Tree"),
    ("internal/storage", "fichiers, écriture atomique, verrous"),
    ("internal/config", "variables d'environnement"),
]):
    node(ML + i * (low_w + 0.22), 5.14, low_w, 0.80, name, sub)
note(s, "Le moteur connaît la ligne du bas ; la ligne du bas ne connaît pas le moteur. "
        "Go refuse les cycles d'import — l'architecture est donc vérifiée par le compilateur.", y=6.18)

for ax in (ML + row_w + 0.22 + row_w / 2, ML + 2 * (row_w + 0.22) + row_w / 2):
    arrow_down(s, ax, 2.58, 0.24)
arrow_down(s, ML + row_w + 0.22 + row_w, 3.64, 0.24)
for i in range(4):
    arrow_down(s, ML + i * (low_w + 0.22) + low_w / 2, 4.84, 0.26, colour="#dde2e0")

# ---------------------------------------------------------------------------
img = code_png("s05_types", "internal/engine/types.go", [(11, 29)],
               badges={12: 1, 13: 2, 14: 3, 17: 4, 18: 5, 26: 6}, font_px=15)
code_slide("Le modèle de données", "Tout l'état du moteur tient dans une structure", img, [
    "`state` est une simple `map` : toutes les clés tiennent en mémoire vive.",
    "Un seul verrou `mu` protège l'ensemble du moteur.",
    "`now` est une *fonction*, pas un appel à `time.Now` en dur : les tests y injectent une horloge qu'ils contrôlent.",
    "`buffer` retient les écritures qui n'ont pas encore été versées au journal.",
    "`sequence` numérote chaque opération — la clé de la reprise après une panne.",
    "Une valeur, c'est *du texte plus un booléen*. Les comparaisons numériques passent par `float64`.",
], footer="Un nombre entre guillemets reste une chaîne. Cette décision, prise une seule fois à l'écriture, "
          "gouverne ensuite toutes les recherches.")

# ================================================= 2. LE CHEMIN D'UN « SET »
divider(2, "Que se passe-t-il quand on fait un SET ?",
        ["On suit une seule ligne de texte — SET age 42 — depuis le clavier jusqu'au disque.",
         "Sept étapes, sept fichiers, et une seule d'entre elles touche vraiment le disque."])

s = slide("La carte", "SET age 42, de bout en bout")
table(s, ML, TOP, CW,
      ["", "Étape", "Où", "Ce qui se passe"],
      [["1", "Découper la ligne", "command/command.go · split", "« SET age 42 » devient trois jetons, en retenant lesquels étaient entre guillemets"],
       ["2", "Reconnaître la commande", "command/command.go · Parse", "Les jetons deviennent une structure Command, ou une erreur"],
       ["3", "Décider du type", "command/command.go · numeric", "42 est-il un nombre ou une chaîne ? La réponse est figée ici"],
       ["4", "Valider et verrouiller", "engine/engine.go · set", "Le moteur revérifie tout, puis prend le verrou"],
       ["5", "Écrire en mémoire", "engine/engine.go + index", "La map et les deux index sont mis à jour ensemble"],
       ["6", "Empiler l'opération", "engine/engine.go · record", "L'opération numérotée entre dans un tampon ; SET peut retourner OK"],
       ["7", "Écrire sur le disque", "storage/storage.go · AppendAOF", "Au plus tard une seconde plus tard, le tampon est versé au journal et synchronisé"]],
      widths=[0.4, 3.0, 3.6, 8.2], size=11.5, head_size=9)
note(s, "Les étapes 1 à 6 ne quittent jamais la mémoire. C'est la septième, différée, "
        "qui sépare « rapide » de « durable ».")

# --- étape 1 ---------------------------------------------------------------
img = code_png("s07_split", "internal/command/command.go", [(96, 113), (122, 124)],
               badges={97: 1, 106: 2, 110: 3, 112: 4, 122: 5}, font_px=14)
code_slide("Étape 1 · le parser", "Découper la ligne sans perdre les guillemets", img, [
    "Tant qu'un guillemet est ouvert, chaque caractère est pris tel quel : c'est ce qui permet d'écrire `SET nom \"Ada Lovelace\"`.",
    "Le guillemet n'est pas conservé dans le texte, mais il laisse une trace : `quoted = true`.",
    "Hors guillemets, une espace ferme le mot courant. C'est ce qui découpe la ligne en jetons.",
    "Chaque jeton retient son texte *et* s'il était entre guillemets. Cette seconde information décidera du type à l'étape 3.",
    "Un guillemet jamais refermé est une erreur franche, pas un mot qui déborde.",
], footer="La branche `default`, omise ici, se contente d'ajouter le caractère au mot en cours. "
          "Les échappements ne sont pas interprétés : c'est un choix de simplicité.")

# --- étape 2 ---------------------------------------------------------------
img = code_png("s08_parse", "internal/command/command.go", [(47, 62)],
               badges={49: 1, 52: 2, 53: 3, 55: 4, 58: 5}, font_px=14)
code_slide("Étape 2 · le parser", "Des jetons à une commande, ou une erreur", img, [
    "Un `SET` a soit trois jetons, soit cinq avec l'expiration. Toute autre longueur est refusée avant d'atteindre le moteur.",
    "La clé et la valeur sont simplement recopiées : le parser ne les transforme pas.",
    "Le type est décidé ici, une fois pour toutes.",
    "`EX` n'accepte que des chiffres : `strings.Trim` retire tous les chiffres et vérifie qu'il ne reste rien — pas de `-5`, pas de `3.5`.",
    "La durée est reconstruite en secondes, puis revalidée : une durée nulle ou négative est refusée.",
], footer="Le parser ne touche jamais aux données. Il transforme du texte en une valeur `Command`, "
          "ou il renvoie une erreur — et rien n'a bougé.")

# --- étape 3 ---------------------------------------------------------------
img = code_png("s09_numeric", "internal/command/command.go", [(81, 87)],
               badges={82: 1, 85: 2, 86: 3}, font_px=15)
s = slide("Étape 3 · le typage", "Une fonction de six lignes décide de tout")
place(s, img, ML, TOP, 6.9, 1.9)
annotations(s, ML, TOP + 2.05, 6.6, [
    "Un jeton qui était entre guillemets est une chaîne, point final.",
    "Sinon, on tente une conversion en `float64`.",
    "`NaN` et l'infini se convertissent sans erreur mais ne sont pas des nombres utilisables : ils redeviennent des chaînes.",
], size=12)
table(s, ML + 7.15, TOP, CW - 7.15,
      ["Commande", "Stocké comme"],
      [["SET age 42", "nombre 42"],
       ["SET age \"42\"", "chaîne « 42 »"],
       ["SET age 4e3", "nombre 4 000"],
       ["SET age NaN", "chaîne « NaN »"],
       ["SET nom Ada", "chaîne « Ada »"]],
      widths=[1.3, 1.0], size=12)
note(s, "Vérifié en vrai : après `SET a 4e3`, la recherche `GET WHERE value >= 3999` trouve la clé — "
        "mais `GET a` rend toujours le texte d'origine, « 4e3 ». Le texte est conservé, le nombre sert à comparer.")

# --- étape 4 ---------------------------------------------------------------
img = code_png("s11_set_a", "internal/engine/engine.go", [(63, 80)],
               badges={64: 1, 67: 2, 73: 3, 74: 4, 78: 5}, highlight=(73,), font_px=14)
code_slide("Étape 4 · le moteur", "Refuser tôt, verrouiller tard", img, [
    "Un TTL négatif n'a pas de sens : refus immédiat.",
    "Si le parser a annoncé un nombre, le moteur le revérifie lui-même — l'API Go est publique et peut être appelée sans passer par le parser.",
    "*Tout ce qui précède se fait hors du verrou.* On ne bloque les autres appels qu'une fois les validations passées.",
    "`defer` garantit le déverrouillage, y compris sur les retours d'erreur qui suivent.",
    "Un `SET` sans `EX` hérite du TTL par défaut de la configuration : zéro, donc aucune expiration, sauf réglage contraire.",
], footer="Valider, puis verrouiller, puis modifier. Une commande destinée à être refusée "
          "n'aura jamais fait attendre les autres.")

# --- étape 5 ---------------------------------------------------------------
img = code_png("s12_set_b", "internal/engine/engine.go", [(81, 94)],
               badges={81: 1, 83: 2, 84: 3, 88: 4, 91: 5, 92: 6}, font_px=14)
code_slide("Étape 5 · le moteur", "Construire l'entrée, puis publier le changement", img, [
    "L'entrée est construite d'un bloc : clé, valeur, type.",
    "L'expiration est une *date absolue*, calculée maintenant — pas une durée qui décompte.",
    "Garde-fou : cette date doit survivre à l'aller-retour en nanosecondes, car c'est sous cette forme qu'elle sera écrite dans le journal.",
    "L'index est mis à jour *avant* la mémoire. S'il échoue, `e.state` n'a pas bougé : jamais d'index désynchronisé des données.",
    "L'écriture elle-même : une affectation dans une map. Un `SET` sur une clé existante la remplace, sans cas particulier.",
    "L'opération est enregistrée pour le journal. À cet instant, elle n'est *pas encore* sur le disque.",
], footer="Trois lignes suffisent à écrire : index, mémoire, journal. L'ordre entre les trois, lui, "
          "n'est pas négociable.")

# --- étape 5 bis -----------------------------------------------------------
img = code_png("s13_index_set", "internal/index/index.go", [(49, 63)],
               badges={50: 1, 54: 2, 56: 3, 59: 4, 61: 5}, font_px=15)
code_slide("Étape 5 · les index", "Une écriture, quatre structures tenues à jour", img, [
    "`scalar` normalise : un nombre est converti puis reformaté. `4e3` et `4000` deviennent la même entrée d'index.",
    "Remplacer, c'est d'abord supprimer. Sans cette ligne, l'ancienne valeur garderait la clé dans son ensemble et `GET WHERE` mentirait.",
    "L'arbre des clés, pour les recherches `GET WHERE key > …`.",
    "Une valeur encore inconnue crée son ensemble et entre dans l'arbre des valeurs.",
    "L'index inversé : valeur → ensemble de clés. `equals` ne sera plus qu'une lecture de map.",
], footer="Quatre structures pour un seul fait — « la clé K vaut V ». C'est le prix d'une recherche rapide, "
          "payé à chaque écriture.")

# --- étape 6 ---------------------------------------------------------------
img = code_png("s14_record", "internal/engine/engine.go", [(137, 147)],
               badges={138: 1, 141: 2, 142: 3, 143: 4, 146: 5}, font_px=15)
code_slide("Étape 6 · le tampon", "L'opération entre dans la file d'attente du disque", img, [
    "En mode `-memory`, il n'y a pas de stockage : la fonction ne fait rien et tout le reste du moteur est identique.",
    "Chaque opération reçoit un numéro qui ne recule jamais.",
    "L'opération journalisée est autosuffisante : elle contient tout ce qu'il faut pour être rejouée seule.",
    "L'expiration devient un entier — des nanosecondes depuis 1970 — parce que c'est ce qui s'écrit proprement en JSON.",
    "Elle est simplement ajoutée au tampon, et `SET` peut retourner `OK`. *L'appelant n'a pas attendu le disque.*",
], footer="C'est la raison pour laquelle un arrêt brutal peut perdre les toutes dernières écritures. "
          "Le compromis est assumé, et documenté.")

# --- étape 7a --------------------------------------------------------------
img = code_png("s15_flush", "internal/engine/persistence.go", [(14, 29)],
               badges={14: 1, 21: 2, 24: 3, 25: 4, 27: 5}, font_px=15)
code_slide("Étape 7 · le flush", "Vider le tampon, ou ne rien perdre en essayant", img, [
    "La version publique prend le verrou, la version privée suppose qu'il est déjà pris. Ce doublet se retrouve partout dans le moteur.",
    "Sans stockage, ou sans rien à écrire, il n'y a rien à faire.",
    "Une seule écriture pour tout le tampon, pas une écriture par opération.",
    "*En cas d'échec, on retourne sans vider le tampon.* Les opérations restent en mémoire et seront réessayées au flush suivant.",
    "Le tampon n'est remis à zéro qu'après un succès confirmé.",
], footer="Une tâche de fond appelle `Flush` toutes les secondes. `Close` en fait un dernier "
          "à l'arrêt, et `POST /api/flush` permet de le déclencher à la demande.")

# --- étape 7b --------------------------------------------------------------
img = code_png("s16_append", "internal/storage/storage.go", [(22, 28), (32, 47)],
               badges={24: 1, 32: 2, 37: 3, 41: 4, 45: 5, 47: 6}, font_px=13.5)
code_slide("Étape 7 · le disque", "Le contrat avec le système de fichiers", img, [
    "Tout le JSON est fabriqué en mémoire d'abord, une ligne par opération. Si l'encodage échoue, le fichier n'a pas été touché.",
    "Ouverture en *ajout* : on n'écrase jamais ce qui est déjà écrit.",
    "On relève la taille du fichier *avant* d'écrire. C'est le point de retour.",
    "`Write` puis `Sync` : c'est `Sync` qui force le système à poser réellement les octets sur le disque.",
    "Si quelque chose échoue, le fichier est retronqué à sa taille d'avant : le journal ne conserve jamais une ligne à moitié écrite.",
    "On synchronise aussi le *dossier* — sinon la création du fichier elle-même pourrait ne pas survivre à une coupure de courant.",
], size=12, footer="C'est ici, et seulement ici, que « la donnée est enregistrée » devient vrai.")

# --- la trace --------------------------------------------------------------
img = term_png("s17_session", [
    ("$ go run . -cli", "cmd"),
    ("restauration : 0 clé en 0s", "dim"),
    ("", "dim"),
    ("> SET nom \"Ada Lovelace\"", "in"),
    ("OK", "out"),
    ("> SET age 42", "in"),
    ("OK", "out"),
    ("> SET etiquette \"42\"", "in"),
    ("OK", "out"),
    ("> SET code secret EX 60", "in"),
    ("OK", "out"),
    ("> GET age", "in"),
    ("42", "out"),
    ("> GET WHERE value >= 21", "in"),
    ("[{\"key\":\"age\",\"value\":\"42\",\"isNumber\":true,\"expiresAt\":\"0001-01-01T00:00:00Z\"}]", "out"),
    ("> DEL etiquette", "in"),
    ("OK", "out"),
    ("> EXIT", "in"),
    ("moteur fermé : 5 opérations écrites au dernier flush", "dim"),
], font_px=13.5)
code_slide("La preuve", "Une vraie session, copiée telle quelle", img, [
    "`GET` et `PING` ne produisent aucune opération : seules les écritures entrent dans le journal.",
    "`GET WHERE value >= 21` rend `age` et *ignore* `etiquette`, dont la valeur « 42 » est une chaîne. Même texte, type différent, résultat différent.",
    "À la fermeture, le moteur annonce cinq opérations écrites : quatre `SET` et un `DEL`.",
    "Les cinq ont attendu dans le tampon ; aucune n'a fait patienter l'utilisateur.",
], max_w=7.6)

img = json_png("s18_aof", '''{"sequence":1,"kind":"SET","key":"nom","value":"Ada Lovelace","isNumber":false,"expiresAt":0}
{"sequence":2,"kind":"SET","key":"age","value":"42","isNumber":true,"expiresAt":0}
{"sequence":3,"kind":"SET","key":"etiquette","value":"42","isNumber":false,"expiresAt":0}
{"sequence":4,"kind":"SET","key":"code","value":"secret","isNumber":false,"expiresAt":1789545818655556325}
{"sequence":5,"kind":"DELETE","key":"etiquette","value":"","isNumber":false,"expiresAt":0}''',
               font_px=14, max_cols=120)
s = slide("La preuve", "Ce que le journal a retenu — data/aof.log")
place(s, img, ML, TOP, CW, 1.5)
annotations(s, ML, TOP + 1.72, CW / 2 - 0.25, [
    "Une ligne = une opération, au format *JSON Lines*. Le retour à la ligne final fait partie du contrat.",
    "Les numéros de séquence se suivent. Un trou signalerait un journal corrompu, et le démarrage échouerait.",
    "`isNumber` sépare `age` de `etiquette` : la même valeur « 42 », écrite deux fois, avec deux types.",
], size=12)
annotations(s, ML + CW / 2 + 0.25, TOP + 1.72, CW / 2 - 0.25, [
    "L'expiration est une date absolue en nanosecondes depuis 1970. Zéro signifie « jamais ».",
    "La suppression est une ligne *en plus*, pas une ligne en moins. Le journal ne se réécrit pas : il s'allonge, jusqu'au prochain snapshot.",
], size=12, start=4)
note(s, "Ce fichier se lit avec `cat`. C'était un objectif : pouvoir vérifier à l'œil ce que le moteur a promis.")

# ============================================ 3. LIRE, CHERCHER, OUBLIER
divider(3, "Lire, chercher, oublier",
        ["GET, DELETE, GET WHERE et l'expiration : le reste du vocabulaire du moteur.",
         "C'est là que le choix de typer les valeurs se paie — et se justifie."])

img = code_png("s20_get", "internal/engine/engine.go", [(96, 119)],
               badges={104: 1, 108: 2, 109: 3, 110: 4, 118: 5}, font_px=13.5)
code_slide("Lire", "GET, et l'oubli paresseux", img, [
    "La lecture, c'est un accès à la map. Rien de plus.",
    "Une clé peut être présente *et* périmée : le test d'expiration se fait au moment de la lecture.",
    "Sauf si le moteur est fermé — on ne journalise plus rien après `Close`.",
    "Sinon la clé périmée est supprimée au passage : la lecture fait le ménage, index et journal compris.",
    "Une date d'expiration nulle signifie « jamais ». À l'instant exact de l'expiration, la clé est déjà considérée comme périmée.",
], footer="Conséquence : le moteur ne rend jamais une clé périmée, même si le balayage de fond "
          "n'est pas encore passé.")

img = code_png("s21_delete", "internal/engine/engine.go", [(121, 135)],
               badges={124: 1, 127: 2, 132: 3, 133: 4, 134: 5}, font_px=15)
code_slide("Supprimer", "DELETE, ou une ligne de plus dans le journal", img, [
    "Après `Close`, on refuse l'écriture plutôt que de la perdre en silence.",
    "`Delete` et l'expiration passent par le même chemin : `remove`.",
    "L'index est nettoyé en premier, comme à l'écriture.",
    "`delete` sur une clé absente ne fait rien. Supprimer ce qui n'existe pas n'est pas une erreur.",
    "La suppression est journalisée comme une opération à part entière, avec son propre numéro de séquence.",
], footer="Le journal ne rétrécit jamais lors d'un `DEL` : il s'allonge d'une ligne qui dit « oublie celle-ci ». "
          "Seul le snapshot fait maigrir le fichier.")

img = code_png("s22_index_types", "internal/index/types.go", [(5, 16)],
               badges={6: 1, 12: 2, 13: 3, 14: 4, 15: 5}, font_px=15)
code_slide("Chercher", "Quatre structures pour ne pas tout parcourir", img, [
    "Une `value` retient son texte *et* son équivalent `float64`, plus le drapeau qui dit lequel fait foi.",
    "`keys` : un B-Tree ordonné des clés, pour les plages sur `key`.",
    "`values` : un B-Tree ordonné des valeurs, pour les plages sur `value`.",
    "`entries` : clé → valeur. Sert à retrouver l'ancienne valeur lors d'un remplacement.",
    "`inverted` : valeur → ensemble des clés. C'est l'index inversé, et `equals` y lit directement.",
], lead="Le problème : trouver toutes les clés dont la valeur dépasse 21, sans lire toutes les clés.",
   footer="Le B-Tree vient de `github.com/google/btree`. Écrire le nôtre aurait triplé la taille du paquet "
          "sans rien apprendre de plus sur le sujet du projet.")

img = code_png("s23_find_eq", "internal/index/index.go", [(95, 101), (134, 137), (144, 145)],
               badges={96: 1, 98: 2, 135: 3, 144: 4}, font_px=14, max_cols=78)
code_slide("Chercher", "equals et contains : les deux extrêmes", img, [
    "`contains` n'a aucun index : il parcourt toutes les entrées, une par une.",
    "Sur une valeur, `contains` ignore les nombres — chercher « 4 » dans 42 n'aurait pas de sens ici.",
    "`equals`, à l'inverse, est une simple lecture de l'index inversé. Son coût dépend du nombre de résultats, pas du nombre de clés.",
    "Tous les chemins finissent triés par clé : le résultat est stable, alors que l'ordre de parcours d'une map en Go ne l'est pas.",
], size=12, footer="Deux extrêmes dans la même fonction. Le coût de `contains` est assumé et écrit dans le README : "
                   "indexer les sous-chaînes aurait coûté bien plus cher à chaque écriture.")

img = code_png("s24_find_range", "internal/index/index.go", [(121, 132), (138, 142)],
               badges={121: 1, 122: 2, 125: 3, 128: 4, 139: 5, 141: 6}, font_px=14)
code_slide("Chercher", "Les plages : descendre dans le B-Tree", img, [
    "`visit` est appelé par l'arbre pour chaque valeur rencontrée, dans l'ordre.",
    "*Dès qu'on change de type, on s'arrête.* Renvoyer `false` interrompt le parcours.",
    "Sur `>` et `<` stricts, la borne elle-même est visitée mais pas retenue.",
    "Pour chaque valeur retenue, on récupère d'un coup toutes ses clés dans l'index inversé.",
    "L'arbre se positionne sur la borne et remonte en ordre croissant, sans jamais regarder ce qui est en dessous.",
    "Et symétriquement pour les bornes inférieures.",
], footer="Sans l'arbre, il faudrait comparer les 100 000 valeurs pour en trouver trois. Avec lui, "
          "on entre à la bonne place et on s'arrête dès que la condition tombe.")

img = code_png("s25_less", "internal/index/index.go", [(23, 31)],
               badges={24: 1, 25: 2, 28: 3, 30: 4}, font_px=15)
trace = term_png("s25_trace", [
    ("> SET age 42", "in"),
    ("> SET etiquette \"42\"", "in"),
    ("> GET WHERE value >= 21", "in"),
    ("[{\"key\":\"age\",\"value\":\"42\",\"isNumber\":true,\"expiresAt\":\"0001-01-01T00:00:00Z\"}]", "out"),
    ("une seule clé : « 42 » entre guillemets est une chaîne", "note"),
], font_px=12.5)
s = slide("La décision structurante", "Nombres et chaînes ne se mélangent pas")
place(s, img, ML, TOP, 6.7, 2.05)
place(s, trace, ML, TOP + 2.25, 6.7, 1.7)
annotations(s, ML + 7.15, TOP + 0.05, CW - 7.15, [
    "Deux valeurs de types différents ne se comparent pas sur leur contenu.",
    "Elles sont rangées en deux blocs : toutes les chaînes d'un côté, tous les nombres de l'autre.",
    "Entre nombres, on compare des `float64`.",
    "Entre chaînes, l'ordre lexicographique.",
], size=12.5)
note(s, "C'est la question que pose toujours un jury : « et si je stocke 42 et \"42\" ? » "
        "Réponse : deux valeurs distinctes, dans deux moitiés distinctes de l'arbre, et une recherche "
        "numérique ne verra jamais la seconde.")

img = code_png("s26_sweep", "internal/engine/background.go", [(9, 24)],
               badges={12: 1, 16: 2, 17: 3, 19: 4}, font_px=15)
code_slide("Oublier", "TTL : deux chemins mènent à l'oubli", img, [
    "Après fermeture, le balayage ne fait plus rien : personne ne pourrait plus écrire son résultat.",
    "L'heure est relevée *une seule fois* pour tout le balayage — deux clés qui expirent en même temps disparaissent ensemble.",
    "Le balayage est complet : toutes les clés, à chaque passage. Simple, et suffisant à cette échelle.",
    "Le même `remove` que `DEL` : index nettoyé et suppression journalisée.",
], footer="La lecture garantit la justesse — elle ne rend jamais une clé périmée. Le balayage, lui, "
          "libère la mémoire des clés que personne ne lit plus. Toutes les dix secondes par défaut.")

# ====================================== 4. SURVIVRE À UN REDÉMARRAGE
divider(4, "Survivre à un redémarrage",
        ["Un journal qui s'allonge, un snapshot qui le remplace, et une reprise qui recolle les deux.",
         "Toute la difficulté tient dans un mot : que se passe-t-il si le courant coupe ici ?"])

img = code_png("s28_snapshot", "internal/engine/persistence.go", [(31, 45), (49, 55)],
               badges={37: 1, 40: 2, 42: 3, 51: 4, 54: 5}, font_px=13.5, max_cols=88)
code_slide("Compacter", "Le snapshot, puis le journal — jamais l'inverse", img, [
    "On commence par vider le tampon : le snapshot doit refléter tout ce qui a déjà été accepté.",
    "Le snapshot retient le numéro de séquence atteint. C'est lui qui dira, au redémarrage, quelles lignes du journal sont déjà couvertes.",
    "Les clés périmées ne sont pas recopiées : le snapshot fait aussi le ménage.",
    "Le fichier est écrit — de façon atomique, on y vient.",
    "*Et seulement ensuite* le journal est vidé.",
], size=12, footer="Si le programme meurt entre l'étape 4 et l'étape 5, le journal contient des lignes déjà "
                   "couvertes par le snapshot. Les numéros de séquence les rendent inoffensives.")

img = code_png("s29_atomic", "internal/storage/storage.go", [(125, 145)],
               badges={129: 1, 133: 2, 134: 3, 137: 4, 141: 5, 144: 6}, font_px=13.5)
code_slide("Écrire sans casser", "Fichier temporaire, synchronisation, renommage", img, [
    "On écrit dans un fichier temporaire placé *à côté* de la cible — même dossier, donc même système de fichiers.",
    "Si quoi que ce soit échoue ensuite, le temporaire est effacé et le fichier visé n'aura jamais été touché.",
    "Écriture puis `Sync` : les octets sont réellement sur le disque avant qu'on aille plus loin.",
    "`errors.Join` conserve les deux erreurs possibles, celle de l'écriture et celle de la fermeture.",
    "`Rename` sur un même système de fichiers est *atomique* : à tout instant, la cible est soit l'ancien fichier entier, soit le nouveau entier.",
    "On synchronise aussi le dossier, pour que le renommage lui-même survive à une coupure.",
], size=12, footer="Jamais de fichier à moitié écrit. C'est ce qui autorise à couper le courant pendant "
                   "un snapshot sans perdre le précédent.")

img = code_png("s30_replay", "internal/engine/persistence.go", [(83, 101)],
               badges={84: 1, 88: 2, 94: 3, 97: 4, 100: 5}, font_px=14)
code_slide("Reprendre", "Le redémarrage : snapshot d'abord, journal ensuite", img, [
    "Le snapshot est déjà chargé. On rejoue maintenant le journal par-dessus.",
    "Les lignes déjà couvertes par le snapshot sont ignorées : c'est le filet de sécurité d'une compaction interrompue.",
    "Un trou dans la numérotation arrête le démarrage. Mieux vaut refuser d'ouvrir que servir des données incomplètes.",
    "`applyOp` refait l'opération : un `SET` réécrit, un `DELETE` efface.",
    "Et le compteur avance, pour que la prochaine écriture reprenne la suite.",
], footer="Après le rejeu, les clés périmées sont éliminées et les index reconstruits de zéro. "
          "Ils ne sont jamais persistés : les recalculer est plus simple, et plus sûr, que les stocker.")

img = code_png("s31_readaof", "internal/storage/storage.go", [(60, 71)],
               badges={60: 1, 62: 2, 67: 3}, font_px=14)
code_slide("Distinguer", "Une écriture interrompue n'est pas une corruption", img, [
    "On ne lit que jusqu'au *dernier retour à la ligne*. Ce qui suit est une écriture coupée en vol : elle est ignorée, puis retirée du fichier.",
    "Chaque ligne complète est une opération.",
    "Une ligne complète mais illisible, elle, est une vraie corruption. Le démarrage échoue en nommant la ligne fautive.",
], footer="La différence est nette : une ligne tronquée est un accident normal de la coupure de courant ; "
          "un octet modifié au milieu d'une ligne complète ne l'est pas.")

s = slide("La question du jury", "Et si ça coupe ici ?")
table(s, ML, TOP, CW,
      ["Ce qui arrive", "Ce qui se passe alors"],
      [["Coupure pendant un flush", "La dernière ligne, incomplète, est retirée au redémarrage. Les précédentes sont intactes."],
       ["Coupure pendant un snapshot", "Le fichier temporaire est abandonné. L'ancien snapshot et le journal restent valides."],
       ["Coupure entre le snapshot et le vidage du journal", "Le journal contient des lignes déjà couvertes : les numéros de séquence les font ignorer."],
       ["Octet corrompu dans une ligne complète", "Le démarrage échoue et nomme la ligne. Pas de démarrage silencieux sur des données fausses."],
       ["Arrêt brutal (kill -9, coupure)", "Les opérations encore dans le tampon sont perdues. C'est la limite assumée du tampon d'une seconde."],
       ["Deux processus sur les mêmes fichiers", "Le second refuse de démarrer : LockFiles pose un verrou flock sur le journal et le snapshot."]],
      widths=[4.4, 9.0], size=12, row_h=0.52)
note(s, "Un arrêt propre — Ctrl-C, SIGTERM — passe par `Engine.Close()`, qui fait un dernier flush "
        "et journalise le nombre d'opérations écrites. Rien n'est perdu.")

# =============================================== 5. AUTOUR DU MOTEUR
divider(5, "Autour du moteur",
        ["Le cycle de vie du programme, l'API HTTP, le tableau de bord et les vérifications.",
         "Ce qui transforme un paquet Go en quelque chose qu'on peut lancer et montrer."])

img = code_png("s33_main", "main.go", [(81, 102)],
               badges={82: 1, 84: 2, 88: 3, 95: 4, 101: 5, 102: 6}, font_px=13.5, max_cols=86)
code_slide("Le cycle de vie", "Démarrer, écouter les signaux, s'arrêter proprement", img, [
    "Le programme s'abonne à `Ctrl-C` et à `SIGTERM`.",
    "Un `context` annulable sert de signal d'arrêt unique pour tout le programme.",
    "Une goroutine attend le signal et annule le contexte. Un message explique ce qui va se passer.",
    "Les tâches de fond — flush, snapshot, expiration — tournent dans leur propre goroutine et remontent leurs erreurs par un canal.",
    "À la sortie, on annule d'abord, pour que les tâches de fond s'arrêtent.",
    "Puis on attend leur erreur éventuelle et on la joint au résultat. Les `defer` ferment ensuite le moteur, puis libèrent les verrous.",
], size=12, footer="L'ordre de l'arrêt est le contraire de celui du démarrage : tâches de fond, "
                   "puis `Engine.Close()` qui écrit le tampon, puis les verrous de fichiers.")

s = slide("La concurrence", "Un seul verrou, et ce qu'il coûte")
half = CW / 2 - 0.22
card(s, ML, TOP, half, 2.42, "Ce qu'on y gagne",
     ["Aucune lecture ne peut voir un état à moitié écrit.",
      "Les index et les données sont toujours d'accord entre eux.",
      "Le raisonnement tient en une phrase : une opération à la fois."], accent=COMPLETED)
card(s, ML + half + 0.44, TOP, half, 2.42, "Ce qu'on y paie",
     ["Deux lectures ne se font pas en parallèle.",
      "Un balayage d'expiration bloque les écritures le temps du passage.",
      "La montée en charge s'arrête au débit d'un seul cœur."], accent=PENDING)
card(s, ML, TOP + 2.67, half, 2.42, "Ce qui est vérifié",
     ["`go test -race ./...` couvre les accès concurrents et les tâches de fond.",
      "Les tests injectent une horloge et un stockage qui simule les pannes disque.",
      "Le tampon, l'état et les index sont observés depuis l'intérieur du paquet."], accent=RUNNING)
card(s, ML + half + 0.44, TOP + 2.67, half, 2.42, "Ce qu'on ferait ensuite",
     ["Un `sync.RWMutex` laisserait les lectures passer ensemble.",
      "Un découpage de la map en tranches réduirait la contention.",
      "Aucun des deux n'était nécessaire au périmètre demandé."], accent=FAINT)

img = code_png("s35_api", "internal/api/api.go", [(84, 95)],
               badges={84: 1, 86: 2, 89: 3, 91: 4}, font_px=14)
code_slide("L'API HTTP", "Une route, et le même moteur que le terminal", img, [
    "Depuis Go 1.22, la méthode fait partie du motif de route : pas de routeur externe à installer.",
    "`read` refuse les corps de plus de 16 Kio, les champs inconnus et le JSON en trop après l'objet.",
    "Le serveur ne réimplémente rien : il passe la chaîne au même `ExecuteString` que le terminal interactif.",
    "Une commande invalide est une faute du client : 400, pas 500. Un échec d'écriture disque, lui, donne bien 500.",
], footer="Routes : `POST /api/command`, `POST /api/batch`, `POST /api/flush`, `POST /api/snapshot`, "
          "plus les six routes des files de tâches. Le reste sert le tableau de bord compilé.")

img = code_png("s36_batch", "internal/engine/batch.go", [(5, 19)],
               badges={6: 1, 8: 2, 13: 3}, font_px=15)
code_slide("Le lot", "Le batch n'est pas une transaction", img, [
    "Le tableau des résultats a exactement la taille du tableau des commandes.",
    "*Une erreur n'interrompt pas les suivantes* : chaque commande a son propre résultat, à sa propre place.",
    "La variante qui prend des chaînes fait le même travail, en passant d'abord par le parser.",
], lead="Envoyer cent commandes en une requête, sans prétendre qu'elles forment un tout indivisible.",
   footer="Ce n'est pas une transaction : un autre client peut écrire entre deux commandes du lot, et "
          "rien n'est annulé en cas d'erreur. La limite est de 1 000 commandes et 16 Kio par requête.")

s = slide("Le tableau de bord", "Des files de tâches, pour montrer le moteur à l'œuvre",
          lead="Files FIFO, quatre statuts, transitions manuelles — et rien qui exécute le contenu des tâches.")
place(s, os.path.join(IMG, "ui-dashboard.png"), ML, TOP + 0.26, 7.55, 4.35, frame=True)
bullets(s, ML + 7.85, TOP + 0.30, CW - 8.05, [
    "Une file, c'est une liste de tâches *et quatre compteurs* tenus à jour à chaque transition : `List` ne parcourt plus jamais les tâches.",
    "Les transitions autorisées tiennent en une condition : en attente → en cours → terminée ou en erreur, et en erreur → en attente.",
    "Le filtrage passe par la liste déroulante ; les compteurs sous le titre sont informatifs.",
    "L'interface interroge l'API toutes les deux secondes, en annulant proprement la requête précédente.",
    "`-demo-tasks N` fixe le nombre de tâches par file — trois files de mille par défaut.",
], size=12)
note(s, "Rappel : ces files vivent en mémoire et disparaissent à l'arrêt du serveur. "
        "Le magasin clé-valeur, lui, est sur le disque.")

img = code_png("s38_virtual", "web/src/TaskList.jsx", [(13, 16), (23, 24), (30, 34), (38, 39)],
               badges={16: 1, 24: 2, 30: 3, 33: 4, 38: 5}, font_px=14, max_cols=74)
strip = crop("ui-liste-strip", "ui-liste", top=0.055, bottom=0.87)
s = slide("Le défilement virtuel", "Afficher mille lignes sans en construire mille")
place(s, img, ML, TOP, 5.95, 3.30)
annotations(s, ML + 6.35, TOP + 0.02, CW - 6.35, [
    "Au-delà d'environ 33 millions de pixels, un navigateur cesse de gérer la hauteur d'un élément. On plafonne en dessous.",
    "Sous ce plafond, la hauteur totale est exacte : une ligne vaut 56 pixels, et le placement est au pixel près.",
    "Au-dessus, la ligne du haut se déduit d'un simple rapport. Le défilement devient plus rapide qu'une ligne par ligne : c'est le compromis assumé.",
    "On ne construit que la fenêtre visible, plus quatre lignes de marge de chaque côté.",
    "Et on ne demande au serveur que la page de cent qui contient cette fenêtre.",
], size=12)
place(s, strip, ML, TOP + 3.54, 8.55, 1.45)
tf = textbox(s, ML + 8.85, TOP + 3.62, CW - 8.85, 1.3)
para(tf, "Après avoir fait défiler jusqu'au fond de la file, la liste montre la 930ᵉ tâche. "
         "Le document, lui, ne contient qu'une trentaine de lignes.",
     size=11.5, colour=MUTED, first=True, spacing=1.25)

s = slide("Vérifier", "Cinq commandes, et ce qu'elles couvrent")
table(s, ML, TOP, 7.00,
      ["Commande", "Ce qu'elle vérifie"],
      [["go test -race ./...", "28 fonctions de test sur 6 paquets, accès concurrents compris"],
       ["go vet ./...", "Les fautes courantes que le compilateur laisse passer"],
       ["go build ./...", "La compilation de tous les paquets"],
       ["npm run build --prefix web", "La construction du tableau de bord"],
       ["npm run test:e2e --prefix web", "4 scénarios navigateur, sur un serveur isolé"]],
      widths=[3.1, 3.9], size=11.5, row_h=0.58)
cx, cwidth = ML + 7.45, CW - 7.60
tf = textbox(s, cx, TOP, cwidth, 0.3)
para(tf, "LES QUATRE SCÉNARIOS NAVIGATEUR", size=9.5, colour=FAINT, bold=True, first=True)
cy = bullets(s, cx, TOP + 0.36, cwidth, [
    "Créer une file et suivre le cycle complet d'une tâche",
    "Virtualiser mille tâches et charger la fin de la file",
    "Afficher une erreur réseau, puis récupérer automatiquement",
    "Vérifier que l'interface reste utilisable sur mobile",
], size=11.5)
tf = textbox(s, cx, cy + 0.22, cwidth, 0.3)
para(tf, "CE QUE LES TESTS GO SAVENT FAIRE", size=9.5, colour=FAINT, bold=True, first=True)
bullets(s, cx, cy + 0.58, cwidth, [
    "Injecter une horloge, pour tester le TTL sans attendre",
    "Simuler une panne disque et vérifier que le tampon est conservé",
    "Tronquer un journal pour rejouer une coupure",
], size=11.5)
note(s, "Les cinq commandes passent au vert sur ce dépôt. Les tests Go restent dans le paquet "
        "qu'ils testent : six fichiers ont besoin de champs non exportés.")

s = slide("Honnêteté", "Ce que ce projet ne fait pas")
half = CW / 2 - 0.22
card(s, ML, TOP, half, 2.62, "Limites assumées du moteur",
     ["Un seul verrou global : pas de lectures parallèles.",
      "Le balayage d'expiration est complet à chaque passage.",
      "`contains` parcourt toutes les entrées, sans index.",
      "Les recherches ne sont pas paginées : elles rendent tout.",
      "Un arrêt brutal perd le tampon de la dernière seconde."], accent=PENDING, body_size=12)
card(s, ML + half + 0.44, TOP, half, 2.62, "Hors périmètre, et c'était prévu",
     ["Pas de protocole réseau Redis, pas d'authentification.",
      "Les files de tâches ne sont pas persistées.",
      "Aucun worker n'exécute le contenu des tâches.",
      "WebAssembly, OPFS, worker navigateur et SDK TypeScript appartiennent aux lots ultérieurs."], accent=FAINT, body_size=12)
tf = textbox(s, ML, TOP + 2.92, CW, 0.9)
para(tf, "Pourquoi les nommer plutôt que les taire ?", size=14, colour=INK, bold=True, first=True, after=8)
para(tf, "Parce que chacune de ces limites est le revers d'un choix explicite, et qu'aucune n'est "
         "un oubli. Le verrou unique achète la simplicité du raisonnement ; le tampon d'une seconde "
         "achète la vitesse d'écriture ; l'absence d'index sur `contains` évite d'alourdir chaque "
         "`SET` pour une fonctionnalité secondaire. Un jury peut discuter le compromis — c'est bien "
         "qu'il y en ait un.", size=13, colour=MUTED, spacing=1.32)

s = slide("Pour finir", "SET age 42, en une image")
flow(s, ML, TOP + 0.10, CW, [
    ("1 · Découper", "split"),
    ("2 · Reconnaître", "Parse"),
    ("3 · Typer", "numeric"),
    ("4 · Valider", "Engine.set"),
], box_h=0.72)
flow(s, ML, TOP + 1.05, CW * 0.74, [
    ("5 · Écrire", "map + index"),
    ("6 · Empiler", "record"),
    ("7 · Synchroniser", "AppendAOF"),
], box_h=0.72)
tf = textbox(s, ML + CW * 0.76, TOP + 1.12, CW * 0.24, 0.7)
para(tf, "les six premières : en mémoire, en microsecondes\nla septième : sur le disque, dans la seconde",
     size=11, colour=FAINT, first=True, spacing=1.3)
hline(s, ML, TOP + 2.25, CW)
for i, (title, body) in enumerate([
    ("Une décision prise une fois",
     "Nombre ou chaîne : le type est fixé à l'écriture, et toutes les recherches en découlent."),
    ("Un ordre qui ne change jamais",
     "Valider, verrouiller, écrire en mémoire, journaliser, synchroniser. Chaque inversion serait un bug."),
    ("Une panne prévue à chaque étape",
     "Ligne tronquée, snapshot interrompu, disque plein : chaque cas a une réponse écrite et testée."),
]):
    x = ML + i * (CW / 3)
    tf = textbox(s, x, TOP + 2.55, CW / 3 - 0.45, 0.4)
    para(tf, title, size=14, colour=ACCENT_DARK, bold=True, first=True)
    tf = textbox(s, x, TOP + 3.00, CW / 3 - 0.45, 1.2)
    para(tf, body, size=12.5, colour=MUTED, first=True, spacing=1.28)
note(s, "Merci. Le code, le README et cette présentation disent la même chose — c'était le but.")
