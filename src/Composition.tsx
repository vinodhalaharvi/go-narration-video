import { AbsoluteFill, useCurrentFrame, useVideoConfig, Audio, staticFile, interpolate } from 'remotion';
import { Highlight } from 'prism-react-renderer';
import schedule from './schedule.json';

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

export const GoWalkthrough: React.FC = () => {
  const frame = useCurrentFrame();
  const { fps } = useVideoConfig();
  const currentSec = frame / fps;

  const activeLine =
    [...schedule].reverse().find((s) => currentSec >= s.startSec)?.line ?? 1;

  const lineHeight = 36;
  const fontStack = 'Monaco, Menlo, "Courier New", monospace';

  // Build keyframes from schedule: at each scheduled second, where should the scroll be?
  // First keyframe is forced to frame 0 so scroll starts settled (not animating in from below).
  const rawKeyframes = schedule.map((s) => ({
    frame: Math.round(s.startSec * fps),
    scroll: Math.max(0, (s.line - 8) * lineHeight),
  }));

  // Ensure first keyframe is at frame 0 (interpolate clamps before first input)
  const keyframes =
    rawKeyframes.length > 0 && rawKeyframes[0].frame > 0
      ? [{ frame: 0, scroll: rawKeyframes[0].scroll }, ...rawKeyframes]
      : rawKeyframes;

  // Dedupe consecutive frames with the same value (interpolate requires strictly increasing)
  const dedupedFrames: number[] = [];
  const dedupedScrolls: number[] = [];
  for (const k of keyframes) {
    if (dedupedFrames.length === 0 || k.frame > dedupedFrames[dedupedFrames.length - 1]) {
      dedupedFrames.push(k.frame);
      dedupedScrolls.push(k.scroll);
    }
  }

  // Need at least 2 points for interpolate; if only one, pin scroll there
  const scrollY =
    dedupedFrames.length >= 2
      ? interpolate(frame, dedupedFrames, dedupedScrolls, {
          extrapolateLeft: 'clamp',
          extrapolateRight: 'clamp',
          easing: (t) =>
            // ease-in-out cubic
            t < 0.5 ? 4 * t * t * t : 1 - Math.pow(-2 * t + 2, 3) / 2,
        })
      : (dedupedScrolls[0] ?? 0);

  return (
    <AbsoluteFill style={{ backgroundColor: '#0D1117', fontFamily: fontStack, color: '#C9D1D9' }}>
      <div
        style={{
          height: 36,
          background: '#161B22',
          color: '#BBBBBB',
          display: 'flex',
          alignItems: 'center',
          padding: '0 16px',
          fontSize: 13,
          borderBottom: '1px solid #1E1E1E',
        }}
      >
        main.go
      </div>

      <div style={{ flex: 1, padding: '24px 0', overflow: 'hidden' }}>
        <div
          style={{
            transform: `translateY(-${scrollY}px)`,
          }}
        >
          <Highlight theme={githubDarkTheme} code={goCode} language="go">
            {({ tokens, getTokenProps }) => (
              <>
                {tokens.map((line, i) => {
                  const lineNum = i + 1;
                  const isActive = lineNum === activeLine;
                  return (
                    <div
                      key={i}
                      style={{
                        display: 'flex',
                        height: lineHeight,
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
                          fontSize: 20,
                          userSelect: 'none',
                          fontFamily: fontStack,
                        }}
                      >
                        {lineNum}
                      </span>
                      <pre
                        style={{
                          margin: 0,
                          fontFamily: fontStack,
                          fontSize: 22,
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

      <Audio src={staticFile('narration.mp3')} />
    </AbsoluteFill>
  );
};
