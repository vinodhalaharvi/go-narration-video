import { Composition } from 'remotion';
import { GoWalkthrough } from './Composition';

export const RemotionRoot: React.FC = () => {
  return (
    <Composition
      id="GoWalkthrough"
      component={GoWalkthrough}
      fps={30}
      width={1920}
      height={1080}
      durationInFrames={750}
    />
  );
};
