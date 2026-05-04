import { AbsoluteFill, useCurrentFrame, useVideoConfig, Audio, staticFile } from 'remotion';
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

const goLandTheme = {
  plain: { color: '#A9B7C6', backgroundColor: '#2B2B2B' },
  styles: [
    { types: ['comment'], style: { color: '#629755', fontStyle: 'italic' as const } },
    { types: ['keyword'], style: { color: '#CC7832', fontWeight: 'bold' as const } },
    { types: ['boolean'], style: { color: '#CC7832', fontWeight: 'bold' as const } },
    { types: ['string', 'char'], style: { color: '#6A8759' } },
    { types: ['number'], style: { color: '#6897BB' } },
    { types: ['function'], style: { color: '#FFC66D' } },
    { types: ['builtin'], style: { color: '#CC7832' } },
    { types: ['class-name'], style: { color: '#FFC66D' } },
    { types: ['constant'], style: { color: '#9876AA' } },
    { types: ['punctuation'], style: { color: '#A9B7C6' } },
    { types: ['operator'], style: { color: '#A9B7C6' } },
    { types: ['variable'], style: { color: '#A9B7C6' } },
    { types: ['property'], style: { color: '#9876AA' } },
    { types: ['namespace'], style: { color: '#A9B7C6' } },
  ],
};

export const GoWalkthrough: React.FC = () => {
  const frame = useCurrentFrame();
  const { fps } = useVideoConfig();
  const currentSec = frame / fps;

  const activeLine =
    [...schedule].reverse().find((s) => currentSec >= s.startSec)?.line ?? 1;

  const lineHeight = 36;
  const targetScroll = Math.max(0, (activeLine - 8) * lineHeight);
  const fontStack = 'Monaco, Menlo, "Courier New", monospace';

  return (
    <AbsoluteFill style={{ backgroundColor: '#2B2B2B', fontFamily: fontStack }}>
      <div
        style={{
          height: 36,
          background: '#3C3F41',
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
            transform: `translateY(-${targetScroll}px)`,
            transition: 'transform 0.6s cubic-bezier(0.4, 0, 0.2, 1)',
          }}
        >
          <Highlight theme={goLandTheme} code={goCode} language="go">
            {({ tokens, getLineProps, getTokenProps }) => (
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
                        background: isActive ? 'rgba(255, 198, 109, 0.12)' : 'transparent',
                        borderLeft: isActive
                          ? '3px solid #FFC66D'
                          : '3px solid transparent',
                      }}
                    >
                      <span
                        style={{
                          color: isActive ? '#FFC66D' : '#606366',
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

