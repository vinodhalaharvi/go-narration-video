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
                        height: LINE_HEIGHT,
                        alignItems: 'center',
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
                        }}
                      >
                        {lineNum}
                      </span>
                      <pre
                        style={{
                          margin: 0,
                          fontFamily: FONT_STACK,
                          fontSize: CODE_FONT_SIZE,
                          whiteSpace: 'pre',
                          color: '#C9D1D9',
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
                        height: LINE_HEIGHT,
                        alignItems: 'center',
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
                        }}
                      >
                        {lineNum}
                      </span>
                      <pre
                        style={{
                          margin: 0,
                          fontFamily: FONT_STACK,
                          fontSize: CODE_FONT_SIZE,
                          whiteSpace: 'pre',
                          color: '#C9D1D9',
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
  if (frame < INTRO_TOTAL_FRAMES && hasIntro) return null;

  const introOffset = hasIntro ? INTRO_TOTAL_FRAMES : 0;
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

const IntroCard: React.FC<{ frame: number }> = ({ frame }) => {
  if (!isShort) return null;
  const icon = (meta as any).introIcon as string | undefined;
  const text = (meta as any).introText as string | undefined;
  if (!icon && !text) return null;
  if (frame > INTRO_TOTAL_FRAMES) return null;

  const opacity = interpolate(
    frame,
    [0, 4, INTRO_HOLD_FRAMES, INTRO_TOTAL_FRAMES],
    [0, 1, 1, 0],
    { extrapolateLeft: 'clamp', extrapolateRight: 'clamp' }
  );

  const scale = interpolate(frame, [0, INTRO_HOLD_FRAMES], [0.95, 1.02], {
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

      <div style={{ flex: 1, position: 'relative' }}>
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

      <Captions currentSec={currentSec} />
      <TitleOverlay frame={frame} />
      <IntroCard frame={frame} />

      <Audio src={staticFile('narration.mp3')} />
    </AbsoluteFill>
  );
};
