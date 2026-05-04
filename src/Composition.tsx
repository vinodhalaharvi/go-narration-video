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
  const targetScroll = Math.max(0, (activeLine - 8) * lineHeight);
  const fontStack = 'Monaco, Menlo, "Courier New", monospace';

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
            transform: `translateY(-${targetScroll}px)`,
            transition: 'transform 0.6s cubic-bezier(0.4, 0, 0.2, 1)',
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
