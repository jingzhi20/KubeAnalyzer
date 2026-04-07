import { useRef, useState, useEffect, useMemo, useCallback } from 'react';

interface EyeBallProps {
  size?: number;
  pupilSize?: number;
  maxDistance?: number;
  eyeColor?: string;
  pupilColor?: string;
  isBlinking?: boolean;
  forceLookX?: number;
  forceLookY?: number;
  isSad?: boolean;
  sadRotate?: number;
}

function EyeBall({
  size = 48,
  pupilSize = 16,
  maxDistance = 10,
  eyeColor = 'white',
  pupilColor = 'black',
  isBlinking = false,
  forceLookX,
  forceLookY,
  isSad = false,
  sadRotate = 0,
}: EyeBallProps) {
  const eyeRef = useRef<HTMLDivElement>(null);
  const [mousePos, setMousePos] = useState({ x: 0, y: 0 });

  const handleMouseMove = useCallback((e: MouseEvent) => {
    setMousePos({ x: e.clientX, y: e.clientY });
  }, []);

  useEffect(() => {
    window.addEventListener('mousemove', handleMouseMove);
    return () => {
      window.removeEventListener('mousemove', handleMouseMove);
    };
  }, [handleMouseMove]);

  const pupilPosition = useMemo(() => {
    if (!eyeRef.current) return { x: 0, y: 0 };

    // If forced look direction is provided, use that instead of mouse tracking
    if (forceLookX !== undefined && forceLookY !== undefined) {
      return { x: forceLookX, y: forceLookY };
    }

    const eye = eyeRef.current.getBoundingClientRect();
    const eyeCenterX = eye.left + eye.width / 2;
    const eyeCenterY = eye.top + eye.height / 2;

    const deltaX = mousePos.x - eyeCenterX;
    const deltaY = mousePos.y - eyeCenterY;
    const distance = Math.min(Math.sqrt(deltaX ** 2 + deltaY ** 2), maxDistance);

    const angle = Math.atan2(deltaY, deltaX);
    const x = Math.cos(angle) * distance;
    const y = Math.sin(angle) * distance;

    return { x, y };
  }, [mousePos, maxDistance, forceLookX, forceLookY]);

  return (
    <div
      ref={eyeRef}
      className={`eyeball ${isSad ? 'eyeball--sad' : ''}`}
      style={{
        width: `${size}px`,
        height: isBlinking ? '2px' : isSad ? `${size * 0.5}px` : `${size}px`,
        backgroundColor: eyeColor,
        borderRadius: isSad ? `0 0 ${size}px ${size}px` : '50%',
        transform: isSad ? `rotate(${sadRotate}deg)` : 'rotate(0deg)',
      }}
    >
      {!isBlinking && (
        <div
          className="pupil"
          style={{
            width: `${pupilSize}px`,
            height: `${pupilSize}px`,
            backgroundColor: pupilColor,
            transform: `translate(${pupilPosition.x}px, ${isSad ? -1 : pupilPosition.y}px)`,
          }}
        />
      )}
    </div>
  );
}

export default EyeBall;
