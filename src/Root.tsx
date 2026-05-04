import { Composition } from 'remotion';
import { GoWalkthrough } from './Composition';
import meta from './meta.json';

const FPS = 30;
const isShort = meta.format === 'short';
const WIDTH = isShort ? 1080 : 1920;
const HEIGHT = isShort ? 1920 : 1080;

export const RemotionRoot: React.FC = () => {
  return (
    <Composition
      id="GoWalkthrough"
      component={GoWalkthrough}
      fps={FPS}
      width={WIDTH}
      height={HEIGHT}
      durationInFrames={Math.ceil(meta.durationSec * FPS)}
    />
  );
};
