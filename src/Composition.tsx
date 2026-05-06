import { AbsoluteFill, useCurrentFrame, useVideoConfig, Audio, staticFile, interpolate } from 'remotion';
import { Highlight } from 'prism-react-renderer';
import schedule from './schedule.json';
import codeFiles from './codeFiles.json';
import meta from './meta.json';
import captions from './captions.json';

// Legacy fallback if codeFiles.json is empty
const goCode = `package main

import "fmt"

// Functor over a slice: lifts A -> B into []A -> []B
func Map[A, B any](xs []A, f func(A) B) []B {
    out := make([]B, len(xs))
    for i, x := range xs {
        out[i] = f(x)
    }
    return out
}

func main() {
    nums := []int{1, 2, 3, 4, 5}
    squared := Map(nums, func(n int) int {
        return n * n
    })
    fmt.Println(squared)
}`;

const githubDarkTheme = {
  plain: { color: '#C9D1D9', backgroundColor: '#0D1117' },
  styles: [
    { types: ['comment', 'prolog', 'doctype', 'cdata'], style: { color: '#8B949E', fontStyle: 'italic' as const } },
    { types: ['keyword', 'boolean', 'atrule'], style: { color: '#FF7B72' } },
    { types: ['string', 'char', 'attr-value', 'regex'], style: { color: '#A5D6FF' } },
    { types: ['number', 'symbol'], style: { color: '#79C0FF' } },
    { types: ['function'], style: { color: '#D2A8FF' } },
    { types: ['builtin'], style: { color: '#FF7B72' } },
    { types: ['class-name', 'maybe-class-name', 'tag'], style: { color: '#FFA657' } },
    { types: ['constant', 'property'], style: { color: '#79C0FF' } },
    { types: ['punctuation', 'operator'], style: { color: '#C9D1D9' } },
    { types: ['variable', 'namespace'], style: { color: '#C9D1D9' } },
  ],
};

const filesMap: Record<string, string> =
  Object.keys(codeFiles).length > 0
    ? (codeFiles as Record<string, string>)
    : { 'main.go': goCode };

const fileNames = Object.keys(filesMap).sort();

const isShort = meta.format === 'short';
const isTypewriter = (meta as any).typewriter === 'line' || (meta as any).typewriter === 'word';
const typewriterGranularity = (meta as any).typewriter as 'line' | 'word' | undefined;

const LINE_HEIGHT = isShort ? 44 : 36;
const CODE_FONT_SIZE = isShort ? 28 : 22;
const LINE_NUM_FONT_SIZE = isShort ? 24 : 20;
const HEADER_HEIGHT = 36;
const TAB_HEIGHT = isShort ? 0 : 40;
const FONT_STACK = 'Monaco, Menlo, "Courier New", monospace';
const VISIBLE_LINE_OFFSET = isShort ? 5 : 8;

const TITLE_FADE_IN_FRAMES = 6;
const TITLE_HOLD_FRAMES = 60;
const TITLE_FADE_OUT_FRAMES = 12;

const INTRO_HOLD_FRAMES = 45;
const INTRO_FADE_OUT_FRAMES = 8;
const INTRO_TOTAL_FRAMES = INTRO_HOLD_FRAMES + INTRO_FADE_OUT_FRAMES;

const CAPTION_BAND_HEIGHT = 200;
const CAPTION_FONT_SIZE = 52;

// Typewriter reveal data from cmd/build
type RevealEntry = {
  file: string;
  lineFrom: number;
  lineTo: number;
  startSec: number;
  endSec: number;
};
const typewriterReveals: RevealEntry[] = ((meta as any).typewriterReveals as RevealEntry[]) || [];

const vizKind = ((meta as any).viz as string) || '';
const vizStartSec = ((meta as any).vizStartSec as number) || 0;

type OutputLine = { text: string; atSec: number };
const outputStyle = ((meta as any).outputStyle as string) || '';
const outputLines: OutputLine[] = ((meta as any).outputLines as OutputLine[]) || [];

const outroIcon = ((meta as any).outroIcon as string) || '';
const outroText = ((meta as any).outroText as string) || '';
const outroStartSec = ((meta as any).outroStartSec as number) || 0;

type Keyframe = { frame: number; scroll: number };

function buildScrollTimeline(filename: string, fps: number): Keyframe[] {
  const entries = schedule.filter((s) => s.file === filename);
  const raw: Keyframe[] = entries.map((s) => ({
    frame: Math.round(s.startSec * fps),
    scroll: Math.max(0, (s.line - VISIBLE_LINE_OFFSET) * LINE_HEIGHT),
  }));
  if (raw.length > 0 && raw[0].frame > 0) {
    raw.unshift({ frame: 0, scroll: raw[0].scroll });
  }
  const out: Keyframe[] = [];
  for (const k of raw) {
    if (out.length === 0 || k.frame > out[out.length - 1].frame) {
      out.push(k);
    }
  }
  return out;
}

const CodePanel: React.FC<{
  filename: string;
  code: string;
  scrollY: number;
  activeLine: number | null;
  opacity: number;
}> = ({ filename, code, scrollY, activeLine, opacity }) => {
  return (
    <div
      style={{
        position: 'absolute',
        inset: 0,
        opacity,
        transition: 'opacity 0.3s ease',
        pointerEvents: 'none',
      }}
    >
      <div style={{ flex: 1, padding: '24px 0', overflow: 'hidden', height: '100%' }}>
        <div style={{ transform: `translateY(-${scrollY}px)` }}>
          <Highlight theme={githubDarkTheme} code={code} language="go">
            {({ tokens, getTokenProps }) => (
              <>
                {tokens.map((line, i) => {
                  const lineNum = i + 1;
                  const isActive = lineNum === activeLine;
                  return (
                    <div
                      key={`${filename}-${i}`}
                      style={{
                        display: 'flex',
                        minHeight: LINE_HEIGHT,
                        alignItems: 'flex-start',
                        paddingTop: isShort ? 4 : 0,
                        paddingBottom: isShort ? 4 : 0,
                        background: isActive ? 'rgba(88, 166, 255, 0.15)' : 'transparent',
                        borderLeft: isActive
                          ? '3px solid #58A6FF'
                          : '3px solid transparent',
                      }}
                    >
                      <span
                        style={{
                          color: isActive ? '#58A6FF' : '#484F58',
                          width: 60,
                          textAlign: 'right',
                          paddingRight: 16,
                          fontSize: LINE_NUM_FONT_SIZE,
                          userSelect: 'none',
                          fontFamily: FONT_STACK,
                          lineHeight: `${LINE_HEIGHT - (isShort ? 8 : 0)}px`,
                          flexShrink: 0,
                        }}
                      >
                        {lineNum}
                      </span>
                      <pre
                        style={{
                          margin: 0,
                          fontFamily: FONT_STACK,
                          fontSize: CODE_FONT_SIZE,
                          whiteSpace: isShort ? 'pre-wrap' : 'pre',
                          wordBreak: isShort ? 'break-word' : 'normal',
                          color: '#C9D1D9',
                          flex: 1,
                          paddingRight: isShort ? 24 : 0,
                          // Hanging indent: when a line wraps, the continuation
                          // is offset from the left so it visually reads as a
                          // continuation rather than a new statement.
                          paddingLeft: isShort ? 24 : 0,
                          textIndent: isShort ? -24 : 0,
                          lineHeight: `${LINE_HEIGHT - (isShort ? 8 : 0)}px`,
                        }}
                      >
                        {line.map((token, key) => (
                          <span key={key} {...getTokenProps({ token })} />
                        ))}
                      </pre>
                    </div>
                  );
                })}
              </>
            )}
          </Highlight>
        </div>
      </div>
    </div>
  );
};

// ============================================================
// TypewriterPanel — for [[mode:typewriter]] walkthroughs.
// Reveals code progressively (line or word granularity) instead of
// highlighting pre-existing code.
// ============================================================

// Compute how much of `text` should be visible at progress 0..1, using a given
// granularity. Returns the visible substring.
function revealAtProgress(text: string, progress: number, granularity: 'line' | 'word'): string {
  if (progress <= 0) return '';
  if (progress >= 1) return text;
  if (granularity === 'word') {
    // Split keeping whitespace so reconstructed string preserves layout
    const parts = text.split(/(\s+)/);
    const wordCount = parts.filter((p) => /\S/.test(p)).length;
    const targetWords = Math.ceil(wordCount * progress);
    let seenWords = 0;
    let out = '';
    for (const p of parts) {
      out += p;
      if (/\S/.test(p)) {
        seenWords++;
        if (seenWords >= targetWords) break;
      }
    }
    return out;
  }
  // line granularity: reveal whole lines proportionally
  const lines = text.split('\n');
  const targetLines = Math.ceil(lines.length * progress);
  return lines.slice(0, targetLines).join('\n');
}

// Build the visible code state at currentSec, for a given file in typewriter mode.
// Returns the partial code string to render.
function typewriterStateForFile(filename: string, fullCode: string, currentSec: number): string {
  // Find all reveals for this file
  const fileReveals = typewriterReveals.filter((r) => r.file === filename);
  if (fileReveals.length === 0) return '';

  const lines = fullCode.split('\n');

  // Determine "fully revealed up to which line" and which (if any) reveal is currently animating
  let revealedThrough = 0; // line index (1-based, inclusive) fully revealed
  let activeReveal: RevealEntry | null = null;

  for (const r of fileReveals) {
    if (currentSec >= r.endSec) {
      // Fully revealed
      revealedThrough = Math.max(revealedThrough, r.lineTo);
    } else if (currentSec >= r.startSec) {
      // This is the active animating reveal. Earlier lines still fully revealed.
      revealedThrough = Math.max(revealedThrough, r.lineFrom - 1);
      activeReveal = r;
      break;
    }
  }

  // Compose: full lines [1..revealedThrough] + animated portion of activeReveal
  const fullPart = lines.slice(0, revealedThrough).join('\n');

  if (!activeReveal) {
    return fullPart;
  }

  // Animate lines [activeReveal.lineFrom .. activeReveal.lineTo]
  const animatedBlock = lines
    .slice(activeReveal.lineFrom - 1, activeReveal.lineTo)
    .join('\n');
  const span = Math.max(activeReveal.endSec - activeReveal.startSec, 0.01);
  const progress = Math.max(0, Math.min(1, (currentSec - activeReveal.startSec) / span));
  const animated = revealAtProgress(
    animatedBlock,
    progress,
    typewriterGranularity || 'line'
  );

  return fullPart + (fullPart ? '\n' : '') + animated;
}

// Cursor: blinks at end of partial code.
const Cursor: React.FC<{ frame: number }> = ({ frame }) => {
  const blink = Math.floor(frame / 15) % 2 === 0;
  return (
    <span
      style={{
        display: 'inline-block',
        width: '0.5em',
        marginLeft: 1,
        background: blink ? '#58A6FF' : 'transparent',
        height: '1em',
        verticalAlign: 'text-bottom',
      }}
    />
  );
};

const TypewriterPanel: React.FC<{
  filename: string;
  fullCode: string;
  currentSec: number;
  frame: number;
  opacity: number;
}> = ({ filename, fullCode, currentSec, frame, opacity }) => {
  const visibleCode = typewriterStateForFile(filename, fullCode, currentSec);
  const visibleLines = visibleCode.split('\n');

  // Auto-scroll: keep the latest line visible
  const totalLinesVisible = visibleLines.length;
  const scrollY = Math.max(0, (totalLinesVisible - VISIBLE_LINE_OFFSET) * LINE_HEIGHT);

  return (
    <div
      style={{
        position: 'absolute',
        inset: 0,
        opacity,
        transition: 'opacity 0.3s ease',
        pointerEvents: 'none',
      }}
    >
      <div style={{ flex: 1, padding: '24px 0', overflow: 'hidden', height: '100%' }}>
        <div style={{ transform: `translateY(-${scrollY}px)` }}>
          <Highlight theme={githubDarkTheme} code={visibleCode} language="go">
            {({ tokens, getTokenProps }) => (
              <>
                {tokens.map((line, i) => {
                  const lineNum = i + 1;
                  const isLastLine = i === tokens.length - 1;
                  return (
                    <div
                      key={`${filename}-tw-${i}`}
                      style={{
                        display: 'flex',
                        minHeight: LINE_HEIGHT,
                        alignItems: 'flex-start',
                        paddingTop: isShort ? 4 : 0,
                        paddingBottom: isShort ? 4 : 0,
                        background: isLastLine ? 'rgba(88, 166, 255, 0.10)' : 'transparent',
                      }}
                    >
                      <span
                        style={{
                          color: '#484F58',
                          width: 60,
                          textAlign: 'right',
                          paddingRight: 16,
                          fontSize: LINE_NUM_FONT_SIZE,
                          userSelect: 'none',
                          fontFamily: FONT_STACK,
                          lineHeight: `${LINE_HEIGHT - (isShort ? 8 : 0)}px`,
                          flexShrink: 0,
                        }}
                      >
                        {lineNum}
                      </span>
                      <pre
                        style={{
                          margin: 0,
                          fontFamily: FONT_STACK,
                          fontSize: CODE_FONT_SIZE,
                          whiteSpace: isShort ? 'pre-wrap' : 'pre',
                          wordBreak: isShort ? 'break-word' : 'normal',
                          color: '#C9D1D9',
                          flex: 1,
                          paddingRight: isShort ? 24 : 0,
                          paddingLeft: isShort ? 24 : 0,
                          textIndent: isShort ? -24 : 0,
                          lineHeight: `${LINE_HEIGHT - (isShort ? 8 : 0)}px`,
                        }}
                      >
                        {line.map((token, key) => (
                          <span key={key} {...getTokenProps({ token })} />
                        ))}
                        {isLastLine && <Cursor frame={frame} />}
                      </pre>
                    </div>
                  );
                })}
              </>
            )}
          </Highlight>
        </div>
      </div>
    </div>
  );
};

// ============================================================
// TreeViz — bottom-pane visualization of the SampleTree.
// Currently hardcoded to match SampleTree() in fold.go since parsing
// Go AST in the renderer is overkill.
// ============================================================

type VizNode = {
  name: string;
  num: number;
  children?: VizNode[];
  isLeaf?: boolean;
};

// Mirrors SampleTree() in walkthrough-typewriter/01-fold/fold.go
const SAMPLE_TREE: VizNode = {
  name: 'config',
  num: 0,
  children: [
    {
      name: 'alpha',
      num: 50,
      children: [{ name: 'alpha', num: 200, isLeaf: true }],
    },
    {
      name: 'beta',
      num: 30,
      children: [{ name: 'beta', num: 80, isLeaf: true }],
    },
    { name: 'gamma', num: 150, isLeaf: true },
  ],
};

// Layout constants for the tree viz
const TREE_NODE_RADIUS = 36;
const TREE_FONT_SIZE = 18;

// Lays out a tree using a simple recursive horizontal centering algorithm.
// Returns a flat list of {x, y, name, num, isLeaf, parent_id}.
type LaidOutNode = {
  id: string;
  parentId: string | null;
  x: number;
  y: number;
  name: string;
  num: number;
  isLeaf: boolean;
};

function layoutTree(root: VizNode, totalWidth: number, totalHeight: number): LaidOutNode[] {
  // Compute leaf count for each subtree → that determines horizontal width
  function leafCount(n: VizNode): number {
    if (!n.children || n.children.length === 0) return 1;
    return n.children.reduce((s, c) => s + leafCount(c), 0);
  }

  function depth(n: VizNode): number {
    if (!n.children || n.children.length === 0) return 1;
    return 1 + Math.max(...n.children.map(depth));
  }

  const totalLeaves = leafCount(root);
  const treeDepth = depth(root);
  const levelGap = totalHeight / (treeDepth + 1);
  const out: LaidOutNode[] = [];

  function place(
    n: VizNode,
    leftX: number,
    rightX: number,
    level: number,
    parentId: string | null,
    idx: number
  ) {
    const cx = (leftX + rightX) / 2;
    const cy = levelGap * (level + 0.5);
    const id = parentId === null ? 'root' : `${parentId}-${idx}-${n.name}`;
    out.push({
      id,
      parentId,
      x: cx,
      y: cy,
      name: n.name,
      num: n.num,
      isLeaf: !n.children || n.children.length === 0,
    });
    if (n.children) {
      let cursor = leftX;
      n.children.forEach((c, i) => {
        const w = (leafCount(c) / totalLeaves) * (rightX - leftX);
        place(c, cursor, cursor + w, level + 1, id, i);
        cursor += w;
      });
    }
  }
  place(root, 0, totalWidth, 0, null, 0);
  return out;
}

const TreeViz: React.FC<{ currentSec: number; height: number }> = ({ currentSec, height }) => {
  if (!isShort || vizKind !== 'tree') return null;

  // Fade in over 0.5s starting at vizStartSec
  const fadeInDur = 0.5;
  const t = currentSec - vizStartSec;
  if (t < -0.1) return null; // not yet
  const opacity = Math.max(0, Math.min(1, t / fadeInDur));

  const width = 1080;
  const padding = 50;
  const usableWidth = width - padding * 2;
  const usableHeight = height - 60;
  const layout = layoutTree(SAMPLE_TREE, usableWidth, usableHeight);

  return (
    <div
      style={{
        position: 'absolute',
        left: 0,
        right: 0,
        bottom: CAPTION_BAND_HEIGHT,
        height,
        background: '#161B22',
        borderTop: '2px solid #30363D',
        opacity,
        zIndex: 5,
      }}
    >
      <div
        style={{
          position: 'absolute',
          top: 8,
          left: 16,
          color: '#8B949E',
          fontFamily: FONT_STACK,
          fontSize: 16,
        }}
      >
        SampleTree
      </div>
      <svg width={width} height={height} style={{ display: 'block' }}>
        <g transform={`translate(${padding}, 30)`}>
          {/* Edges first so they're under nodes */}
          {layout.map((n) => {
            if (!n.parentId) return null;
            const parent = layout.find((p) => p.id === n.parentId);
            if (!parent) return null;
            return (
              <line
                key={`e-${n.id}`}
                x1={parent.x}
                y1={parent.y}
                x2={n.x}
                y2={n.y}
                stroke="#30363D"
                strokeWidth={2}
              />
            );
          })}
          {/* Nodes */}
          {layout.map((n) => (
            <g key={`n-${n.id}`}>
              <circle
                cx={n.x}
                cy={n.y}
                r={TREE_NODE_RADIUS}
                fill={n.isLeaf ? '#1F6FEB' : '#30363D'}
                stroke="#58A6FF"
                strokeWidth={2}
              />
              <text
                x={n.x}
                y={n.y - 4}
                textAnchor="middle"
                fontFamily={FONT_STACK}
                fontSize={TREE_FONT_SIZE - 4}
                fontWeight="bold"
                fill="#FFFFFF"
              >
                {n.name}
              </text>
              <text
                x={n.x}
                y={n.y + 14}
                textAnchor="middle"
                fontFamily={FONT_STACK}
                fontSize={TREE_FONT_SIZE - 6}
                fill="#FFA657"
              >
                {n.num}
              </text>
            </g>
          ))}
        </g>
      </svg>
    </div>
  );
};

// ============================================================
// OutputPanel — terminal-style text appearing line-by-line synced
// to narration. Reusable for curl output, test results, log lines.
// Shorts only.
// ============================================================

type OutputStyleProps = {
  bg: string;
  fg: string;
  border: string;
  prompt: string; // optional prefix prepended to each line (rare)
  fontSize: number;
};

function styleForOutput(kind: string): OutputStyleProps {
  switch (kind) {
    case 'test':
      return { bg: '#0D1117', fg: '#7CE38B', border: '#238636', prompt: '', fontSize: 26 };
    case 'log':
      return { bg: '#1C1C1C', fg: '#C9D1D9', border: '#484F58', prompt: '', fontSize: 24 };
    case 'terminal':
    default:
      return { bg: '#0D1117', fg: '#7EE787', border: '#1F6FEB', prompt: '', fontSize: 28 };
  }
}

// Color a single output line based on its content (for `test` style only).
function colorizeLine(text: string, style: OutputStyleProps): string {
  if (text.startsWith('FAIL') || text.includes('--- FAIL')) return '#F85149';
  if (text.startsWith('PASS') || text.startsWith('ok ') || text.includes('--- PASS')) return '#7CE38B';
  return style.fg;
}

const OutputPanel: React.FC<{ currentSec: number; height: number }> = ({ currentSec, height }) => {
  if (!isShort || !outputStyle) return null;

  const style = styleForOutput(outputStyle);

  // Determine which lines are visible at currentSec
  const visible = outputLines.filter((l) => currentSec >= l.atSec - 0.05);
  if (visible.length === 0 && currentSec < (outputLines[0]?.atSec ?? Infinity) - 0.5) {
    // Panel is empty and first line hasn't approached yet — don't render anything
    // (keeps the panel hidden until needed)
    return null;
  }

  // Fade in 0.4s before first line appears
  const firstAt = outputLines[0]?.atSec ?? 0;
  const fadeT = currentSec - (firstAt - 0.4);
  const opacity = Math.max(0, Math.min(1, fadeT / 0.4));

  // Auto-scroll: keep latest lines visible. Simple approach — only show last N lines.
  const maxLines = Math.floor((height - 60) / (style.fontSize + 12));
  const shown = visible.slice(-maxLines);

  return (
    <div
      style={{
        position: 'absolute',
        left: 0,
        right: 0,
        bottom: CAPTION_BAND_HEIGHT,
        height,
        background: style.bg,
        borderTop: `2px solid ${style.border}`,
        opacity,
        zIndex: 5,
        padding: '24px 32px',
        boxSizing: 'border-box',
        overflow: 'hidden',
      }}
    >
      <div
        style={{
          color: '#8B949E',
          fontFamily: FONT_STACK,
          fontSize: 16,
          marginBottom: 12,
        }}
      >
        {outputStyle === 'terminal' ? '$ terminal' : outputStyle === 'test' ? 'go test' : 'output'}
      </div>
      <div
        style={{
          fontFamily: FONT_STACK,
          fontSize: style.fontSize,
          lineHeight: `${style.fontSize + 12}px`,
          whiteSpace: 'pre-wrap',
          wordBreak: 'break-word',
        }}
      >
        {shown.map((line, i) => (
          <div
            key={i}
            style={{
              color: colorizeLine(line.text, style),
              opacity: 1,
            }}
          >
            {style.prompt}{line.text || '\u00A0'}
          </div>
        ))}
      </div>
    </div>
  );
};

// ============================================================
// OutroCard — fullscreen card at the end (mirrors IntroCard).
// ============================================================
const OutroCard: React.FC<{ currentSec: number; frame: number }> = ({ currentSec, frame }) => {
  if (!isShort) return null;
  if (!outroIcon && !outroText) return null;
  if (currentSec < outroStartSec - 0.1) return null;

  // Hold for 3.5s, with 0.4s fade in and 0.4s fade out at the end
  const holdDur = 3.5;
  const fadeIn = 0.4;
  const fadeOut = 0.4;
  const t = currentSec - outroStartSec;
  if (t > holdDur) return null;

  let opacity = 1;
  if (t < fadeIn) {
    opacity = t / fadeIn;
  } else if (t > holdDur - fadeOut) {
    opacity = (holdDur - t) / fadeOut;
  }

  // Subtle scale in for visual life
  const scale = 0.95 + Math.min(0.07, t * 0.04);

  return (
    <div
      style={{
        position: 'absolute',
        inset: 0,
        background: 'linear-gradient(135deg, #0D1117 0%, #1C2128 50%, #0D1117 100%)',
        display: 'flex',
        flexDirection: 'column',
        alignItems: 'center',
        justifyContent: 'center',
        opacity,
        zIndex: 100,
        pointerEvents: 'none',
        transform: `scale(${scale})`,
      }}
    >
      {outroIcon && (
        <div
          style={{
            fontSize: 280,
            lineHeight: 1,
            marginBottom: 40,
            filter: 'drop-shadow(0 12px 24px rgba(88, 166, 255, 0.4))',
          }}
        >
          {outroIcon}
        </div>
      )}
      {outroText && (
        <div
          style={{
            color: '#58A6FF',
            fontSize: 80,
            fontWeight: 900,
            fontFamily: 'system-ui, -apple-system, sans-serif',
            textAlign: 'center',
            lineHeight: 1.15,
            padding: '0 60px',
            letterSpacing: -1,
            textShadow: '0 4px 24px rgba(88, 166, 255, 0.5), 0 2px 8px rgba(0, 0, 0, 0.8)',
          }}
        >
          {outroText}
        </div>
      )}
    </div>
  );
};

const TabBar: React.FC<{ activeFile: string }> = ({ activeFile }) => {
  if (fileNames.length <= 1 || isShort) return null;
  return (
    <div
      style={{
        height: TAB_HEIGHT,
        background: '#161B22',
        display: 'flex',
        alignItems: 'flex-end',
        borderBottom: '1px solid #30363D',
        paddingLeft: 16,
        flexShrink: 0,
      }}
    >
      {fileNames.map((name) => {
        const isActive = name === activeFile;
        return (
          <div
            key={name}
            style={{
              padding: '8px 18px',
              fontFamily: FONT_STACK,
              fontSize: 14,
              color: isActive ? '#C9D1D9' : '#7D8590',
              background: isActive ? '#0D1117' : 'transparent',
              borderTop: isActive ? '2px solid #58A6FF' : '2px solid transparent',
              borderLeft: '1px solid #30363D',
              borderRight: '1px solid #30363D',
              transition: 'all 0.3s ease',
              marginRight: -1,
            }}
          >
            {name}
          </div>
        );
      })}
    </div>
  );
};

const TitleOverlay: React.FC<{ frame: number }> = ({ frame }) => {
  if (!isShort || !meta.title) return null;

  const hasIntro = (meta as any).introIcon || (meta as any).introText;

  // Match the dynamic intro duration computed in IntroCard.
  const introUntilSec = ((meta as any).introUntilSec as number) || 0;
  const fps = 30;
  const dynamicIntroFrames = introUntilSec > 0.5
    ? Math.round(introUntilSec * fps)
    : INTRO_TOTAL_FRAMES;

  if (frame < dynamicIntroFrames && hasIntro) return null;

  const introOffset = hasIntro ? dynamicIntroFrames : 0;
  const localFrame = frame - introOffset;
  const fadeInEnd = TITLE_FADE_IN_FRAMES;
  const fadeOutStart = fadeInEnd + TITLE_HOLD_FRAMES;
  const fadeOutEnd = fadeOutStart + TITLE_FADE_OUT_FRAMES;
  if (localFrame > fadeOutEnd) return null;

  const opacity = interpolate(
    localFrame,
    [0, fadeInEnd, fadeOutStart, fadeOutEnd],
    [0, 1, 1, 0],
    { extrapolateLeft: 'clamp', extrapolateRight: 'clamp' }
  );

  return (
    <div
      style={{
        position: 'absolute',
        top: '20%',
        left: 0,
        right: 0,
        display: 'flex',
        justifyContent: 'center',
        opacity,
        pointerEvents: 'none',
        zIndex: 10,
      }}
    >
      <div
        style={{
          background: 'rgba(13, 17, 23, 0.92)',
          color: '#FFA657',
          padding: '20px 36px',
          borderRadius: 12,
          fontSize: 56,
          fontWeight: 700,
          fontFamily: 'system-ui, -apple-system, sans-serif',
          textAlign: 'center',
          maxWidth: '85%',
          border: '2px solid #FFA657',
          boxShadow: '0 8px 32px rgba(0, 0, 0, 0.5)',
        }}
      >
        {meta.title}
      </div>
    </div>
  );
};

// IntroCard renders a fullscreen intro at the very start.
// Duration: meta.introUntilSec (set by build pipeline to first reveal's start
// time in typewriter mode, so any introductory narration plays over it).
// Falls back to ~1.5s if meta.introUntilSec is unset.
const IntroCard: React.FC<{ frame: number }> = ({ frame }) => {
  if (!isShort) return null;
  const icon = (meta as any).introIcon as string | undefined;
  const text = (meta as any).introText as string | undefined;
  if (!icon && !text) return null;

  // Dynamic intro duration: hold for (introUntilSec - 0.3s fade) seconds,
  // then fade out over 0.3s. Fall back to fixed frame counts if no introUntilSec.
  const introUntilSec = ((meta as any).introUntilSec as number) || 0;
  const fps = 30;
  let holdFrames: number;
  let fadeOutFrames: number;
  if (introUntilSec > 0.5) {
    fadeOutFrames = 9; // 0.3s
    holdFrames = Math.max(0, Math.round(introUntilSec * fps) - fadeOutFrames);
  } else {
    holdFrames = INTRO_HOLD_FRAMES;
    fadeOutFrames = INTRO_FADE_OUT_FRAMES;
  }
  const totalFrames = holdFrames + fadeOutFrames;
  if (frame > totalFrames) return null;

  const opacity = interpolate(
    frame,
    [0, 4, holdFrames, totalFrames],
    [0, 1, 1, 0],
    { extrapolateLeft: 'clamp', extrapolateRight: 'clamp' }
  );

  const scale = interpolate(frame, [0, holdFrames], [0.95, 1.02], {
    extrapolateLeft: 'clamp',
    extrapolateRight: 'clamp',
  });

  return (
    <div
      style={{
        position: 'absolute',
        inset: 0,
        background: 'linear-gradient(135deg, #0D1117 0%, #1C2128 50%, #0D1117 100%)',
        display: 'flex',
        flexDirection: 'column',
        alignItems: 'center',
        justifyContent: 'center',
        opacity,
        zIndex: 100,
        pointerEvents: 'none',
        transform: `scale(${scale})`,
      }}
    >
      {icon && (
        <div
          style={{
            fontSize: 320,
            lineHeight: 1,
            marginBottom: 40,
            filter: 'drop-shadow(0 12px 24px rgba(255, 166, 87, 0.4))',
          }}
        >
          {icon}
        </div>
      )}
      {text && (
        <div
          style={{
            color: '#FFA657',
            fontSize: 96,
            fontWeight: 900,
            fontFamily: 'system-ui, -apple-system, sans-serif',
            textAlign: 'center',
            lineHeight: 1.1,
            padding: '0 60px',
            letterSpacing: -1,
            textShadow: '0 4px 24px rgba(255, 166, 87, 0.5), 0 2px 8px rgba(0, 0, 0, 0.8)',
          }}
        >
          {text}
        </div>
      )}
    </div>
  );
};

type Word = { word: string; start: number; end: number };

const Captions: React.FC<{ currentSec: number }> = ({ currentSec }) => {
  if (!isShort) return null;
  const words = captions as Word[];
  if (words.length === 0) return null;

  const currentIdx = words.findIndex((w) => currentSec >= w.start && currentSec < w.end);
  const idx =
    currentIdx >= 0 ? currentIdx : words.findIndex((w) => w.start > currentSec) - 1;
  if (idx < 0) return null;

  const start = Math.max(0, idx - 2);
  const end = Math.min(words.length, start + 5);
  const visible = words.slice(start, end);

  return (
    <div
      style={{
        position: 'absolute',
        bottom: 0,
        left: 0,
        right: 0,
        height: CAPTION_BAND_HEIGHT,
        background:
          'linear-gradient(to bottom, rgba(0,0,0,0) 0%, rgba(0,0,0,0.85) 40%, rgba(0,0,0,0.95) 100%)',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        pointerEvents: 'none',
        zIndex: 5,
      }}
    >
      <div
        style={{
          maxWidth: '90%',
          textAlign: 'center',
          fontFamily: 'system-ui, -apple-system, sans-serif',
          fontSize: CAPTION_FONT_SIZE,
          fontWeight: 700,
          lineHeight: 1.2,
          textShadow: '0 2px 8px rgba(0, 0, 0, 0.8)',
        }}
      >
        {visible.map((w, i) => {
          const wordIdx = start + i;
          const isCurrent = wordIdx === idx;
          return (
            <span
              key={wordIdx}
              style={{
                color: isCurrent ? '#FFD700' : '#FFFFFF',
                marginRight: 12,
                transition: 'color 0.1s ease',
              }}
            >
              {w.word.trim()}
            </span>
          );
        })}
      </div>
    </div>
  );
};

export const GoWalkthrough: React.FC = () => {
  const frame = useCurrentFrame();
  const { fps } = useVideoConfig();
  const currentSec = frame / fps;

  const currentEntry = [...schedule].reverse().find((s) => currentSec >= s.startSec);
  const activeFile = currentEntry?.file ?? fileNames[0];

  const scrollByFile: Record<string, number> = {};
  const activeLineByFile: Record<string, number | null> = {};

  for (const name of fileNames) {
    const kf = buildScrollTimeline(name, fps);
    if (kf.length >= 2) {
      scrollByFile[name] = interpolate(
        frame,
        kf.map((k) => k.frame),
        kf.map((k) => k.scroll),
        {
          extrapolateLeft: 'clamp',
          extrapolateRight: 'clamp',
          easing: (t) =>
            t < 0.5 ? 4 * t * t * t : 1 - Math.pow(-2 * t + 2, 3) / 2,
        }
      );
    } else if (kf.length === 1) {
      scrollByFile[name] = kf[0].scroll;
    } else {
      scrollByFile[name] = 0;
    }

    const fileEntries = schedule.filter((s) => s.file === name);
    const lastForFile = [...fileEntries].reverse().find((s) => currentSec >= s.startSec);
    activeLineByFile[name] = lastForFile?.line ?? null;
  }

  return (
    <AbsoluteFill style={{ backgroundColor: '#0D1117', fontFamily: FONT_STACK, color: '#C9D1D9' }}>
      <div
        style={{
          height: HEADER_HEIGHT,
          background: '#161B22',
          color: '#BBBBBB',
          display: 'flex',
          alignItems: 'center',
          padding: '0 16px',
          fontSize: 13,
          borderBottom: '1px solid #1E1E1E',
          flexShrink: 0,
        }}
      >
        {fileNames.length > 1 && !isShort ? 'Walkthrough' : activeFile}
      </div>

      <TabBar activeFile={activeFile} />

      {(() => {
        // When tree viz or output panel is active, code area shrinks to leave room.
        // Both bottom panels use the same 600px region (one or the other, not both).
        const bottomPanelHeight = (isShort && (vizKind === 'tree' || outputStyle !== '')) ? 600 : 0;
        const bottomPanelVisible =
          (vizKind === 'tree' && currentSec >= vizStartSec - 0.1) ||
          (outputStyle !== '' && outputLines.length > 0 && currentSec >= (outputLines[0]?.atSec ?? Infinity) - 0.4);
        const codeAreaStyle: React.CSSProperties = bottomPanelVisible
          ? { height: `calc(100% - ${bottomPanelHeight}px - ${CAPTION_BAND_HEIGHT}px - ${HEADER_HEIGHT}px)`, position: 'relative', flexShrink: 0 }
          : { flex: 1, position: 'relative' };
        return (
          <div style={codeAreaStyle}>
            {fileNames.map((name) =>
              isTypewriter ? (
                <TypewriterPanel
                  key={name}
                  filename={name}
                  fullCode={filesMap[name]}
                  currentSec={currentSec}
                  frame={frame}
                  opacity={name === activeFile ? 1 : 0}
                />
              ) : (
                <CodePanel
                  key={name}
                  filename={name}
                  code={filesMap[name]}
                  scrollY={scrollByFile[name] ?? 0}
                  activeLine={name === activeFile ? activeLineByFile[name] : null}
                  opacity={name === activeFile ? 1 : 0}
                />
              )
            )}
          </div>
        );
      })()}

      <TreeViz currentSec={currentSec} height={600} />
      <OutputPanel currentSec={currentSec} height={600} />

      <Captions currentSec={currentSec} />
      <TitleOverlay frame={frame} />
      <IntroCard frame={frame} />
      <OutroCard currentSec={currentSec} frame={frame} />

      <Audio src={staticFile('narration.mp3')} />
    </AbsoluteFill>
  );
};
