import { Composition } from 'remotion';
import { GoWalkthrough } from './Composition';
import meta from './meta.json';

const FPS = 30;

export const RemotionRoot: React.FC = () => {
  return (
    <Composition
      id="GoWalkthrough"
      component={GoWalkthrough}
      fps={FPS}
      width={1920}
      height={1080}
      durationInFrames={Math.ceil(meta.durationSec * FPS)}
    />
  );
};
