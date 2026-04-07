import { useRef, useState, useEffect, useCallback, useMemo } from 'react';
import EyeBall from './EyeBall';
import Pupil from './Pupil';
import './AnimatedCharacters.css';

interface AnimatedCharactersProps {
  isTyping: boolean;
  showPassword: boolean;
  passwordLength: number;
  loginFailed: boolean;
  loginSuccess: boolean;
}

interface CharacterPosition {
  faceX: number;
  faceY: number;
  bodySkew: number;
}

interface CharacterCenter {
  x: number;
  y: number;
}

function AnimatedCharacters({
  isTyping,
  showPassword,
  passwordLength,
  loginFailed,
  loginSuccess,
}: AnimatedCharactersProps) {
  const purpleRef = useRef<HTMLDivElement>(null);
  const blackRef = useRef<HTMLDivElement>(null);
  const orangeRef = useRef<HTMLDivElement>(null);
  const yellowRef = useRef<HTMLDivElement>(null);

  const [hasEntered, setHasEntered] = useState(false);
  const [_mousePos, setMousePos] = useState({ x: 0, y: 0 });
  const [isPurpleBlinking, setIsPurpleBlinking] = useState(false);
  const [isBlackBlinking, setIsBlackBlinking] = useState(false);
  const [isOrangeBlinking, setIsOrangeBlinking] = useState(false);
  const [isYellowBlinking, setIsYellowBlinking] = useState(false);
  const [isLookingAtEachOther, setIsLookingAtEachOther] = useState(false);
  const [isPurplePeeking, setIsPurplePeeking] = useState(false);
  const [showConfetti, setShowConfetti] = useState(false);
  const [confettiStyles, setConfettiStyles] = useState<React.CSSProperties[]>([]);
  const [successLookY, setSuccessLookY] = useState(-5);

  const [purplePos, setPurplePos] = useState<CharacterPosition>({ faceX: 0, faceY: 0, bodySkew: 0 });
  const [blackPos, setBlackPos] = useState<CharacterPosition>({ faceX: 0, faceY: 0, bodySkew: 0 });
  const [orangePos, setOrangePos] = useState<CharacterPosition>({ faceX: 0, faceY: 0, bodySkew: 0 });
  const [yellowPos, setYellowPos] = useState<CharacterPosition>({ faceX: 0, faceY: 0, bodySkew: 0 });

  const [characterCenters, setCharacterCenters] = useState<Record<string, CharacterCenter>>({
    purple: { x: 0, y: 0 },
    black: { x: 0, y: 0 },
    orange: { x: 0, y: 0 },
    yellow: { x: 0, y: 0 },
  });

  const isHidingPassword = useMemo(() => passwordLength > 0 && !showPassword, [passwordLength, showPassword]);

  // Generate confetti
  const generateConfetti = useCallback(() => {
    const confettiColors = ['#FF6B6B', '#4ECDC4', '#FFE66D', '#A78BFA', '#FF9B6B', '#6BCB77', '#4D96FF'];
    const styles = Array.from({ length: 180 }, (_, i) => {
      const color = confettiColors[i % confettiColors.length];
      return {
        left: `${Math.random() * 100}%`,
        top: `-${10 + Math.random() * 30}%`,
        backgroundColor: color,
        width: `${4 + Math.random() * 6}px`,
        height: `${8 + Math.random() * 12}px`,
        animationDelay: `${Math.random() * 2}s`,
        animationDuration: `${4.5 + Math.random() * 2}s`,
        transform: `rotate(${Math.random() * 360}deg)`,
      };
    });
    setConfettiStyles(styles);
    setShowConfetti(true);

    setTimeout(() => {
      setShowConfetti(false);
      setConfettiStyles([]);
    }, 8000);
  }, []);

  // Animate success look
  useEffect(() => {
    if (loginSuccess) {
      generateConfetti();
      setSuccessLookY(-5);

      const startY = -5;
      const endY = 4;
      const duration = 5500;
      const startTime = performance.now();

      const step = (now: number) => {
        const elapsed = now - startTime;
        const progress = Math.min(elapsed / duration, 1);
        const eased = progress < 0.5
          ? 4 * progress * progress * progress
          : 1 - Math.pow(-2 * progress + 2, 3) / 2;
        setSuccessLookY(startY + (endY - startY) * eased);
        if (progress < 1) {
          requestAnimationFrame(step);
        }
      };
      requestAnimationFrame(step);
    } else {
      setSuccessLookY(-5);
    }
  }, [loginSuccess, generateConfetti]);

  // Update character centers
  const updateCharacterCenters = useCallback(() => {
    const centers: Record<string, CharacterCenter> = { ...characterCenters };

    if (purpleRef.current) {
      const rect = purpleRef.current.getBoundingClientRect();
      centers.purple = { x: rect.left + rect.width / 2, y: rect.top + rect.height / 3 };
    }
    if (blackRef.current) {
      const rect = blackRef.current.getBoundingClientRect();
      centers.black = { x: rect.left + rect.width / 2, y: rect.top + rect.height / 3 };
    }
    if (orangeRef.current) {
      const rect = orangeRef.current.getBoundingClientRect();
      centers.orange = { x: rect.left + rect.width / 2, y: rect.top + rect.height / 3 };
    }
    if (yellowRef.current) {
      const rect = yellowRef.current.getBoundingClientRect();
      centers.yellow = { x: rect.left + rect.width / 2, y: rect.top + rect.height / 3 };
    }

    setCharacterCenters(centers);
  }, [characterCenters]);

  // Calculate position
  const calculatePosition = useCallback((
    centerX: number,
    centerY: number,
    mx: number,
    my: number,
    rangeX = 15,
    rangeY = 10,
    minX: number | null = null,
    maxX: number | null = null,
    minY: number | null = null,
    maxY: number | null = null
  ): CharacterPosition => {
    const rMinX = minX !== null ? minX : -rangeX;
    const rMaxX = maxX !== null ? maxX : rangeX;
    const rMinY = minY !== null ? minY : -rangeY;
    const rMaxY = maxY !== null ? maxY : rangeY;

    const deltaX = mx - centerX;
    const deltaY = my - centerY;

    const scaleX = Math.max(Math.abs(rMinX), Math.abs(rMaxX));
    const scaleY = Math.max(Math.abs(rMinY), Math.abs(rMaxY));
    const faceX = Math.max(rMinX, Math.min(rMaxX, deltaX / (300 / scaleX)));
    const faceY = Math.max(rMinY, Math.min(rMaxY, deltaY / (300 / scaleY)));
    const bodySkew = Math.max(-6, Math.min(6, -deltaX / 120));

    return { faceX, faceY, bodySkew };
  }, []);

  // Mouse move handler with RAF throttling
  const rafIdRef = useRef<number | null>(null);
  const needsUpdateRef = useRef(false);
  const pendingMouseRef = useRef({ x: 0, y: 0 });

  const updatePositions = useCallback(() => {
    if (needsUpdateRef.current && hasEntered) {
      needsUpdateRef.current = false;
      const { x, y } = pendingMouseRef.current;
      setMousePos({ x, y });

      const { purple, black, orange, yellow } = characterCenters;
      setPurplePos(calculatePosition(purple.x, purple.y, x, y, 0, 0, -46, 18, -8, 5));
      setBlackPos(calculatePosition(black.x, black.y, x, y));
      setOrangePos(calculatePosition(orange.x, orange.y, x, y, 0, 0, -46, 20, -18, 20));
      setYellowPos(calculatePosition(yellow.x, yellow.y, x, y));
    }
    rafIdRef.current = requestAnimationFrame(updatePositions);
  }, [hasEntered, characterCenters, calculatePosition]);

  const handleMouseMove = useCallback((e: MouseEvent) => {
    pendingMouseRef.current = { x: e.clientX, y: e.clientY };
    needsUpdateRef.current = true;
  }, []);

  // Blinking schedules
  const scheduleBlink = useCallback((setter: React.Dispatch<React.SetStateAction<boolean>>, callback: () => void) => {
    const interval = Math.random() * 4000 + 3000;
    const timeout = setTimeout(() => {
      setter(true);
      setTimeout(() => {
        setter(false);
        callback();
      }, 150);
    }, interval);
    return timeout;
  }, []);

  // Purple blinking
  useEffect(() => {
    let timeout: ReturnType<typeof setTimeout>;
    const schedule = () => {
      timeout = scheduleBlink(setIsPurpleBlinking, schedule);
    };
    schedule();
    return () => clearTimeout(timeout);
  }, [scheduleBlink]);

  // Black blinking
  useEffect(() => {
    let timeout: ReturnType<typeof setTimeout>;
    const schedule = () => {
      timeout = scheduleBlink(setIsBlackBlinking, schedule);
    };
    schedule();
    return () => clearTimeout(timeout);
  }, [scheduleBlink]);

  // Orange blinking
  useEffect(() => {
    let timeout: ReturnType<typeof setTimeout>;
    const schedule = () => {
      timeout = scheduleBlink(setIsOrangeBlinking, schedule);
    };
    schedule();
    return () => clearTimeout(timeout);
  }, [scheduleBlink]);

  // Yellow blinking
  useEffect(() => {
    let timeout: ReturnType<typeof setTimeout>;
    const schedule = () => {
      timeout = scheduleBlink(setIsYellowBlinking, schedule);
    };
    schedule();
    return () => clearTimeout(timeout);
  }, [scheduleBlink]);

  // Looking at each other when typing
  useEffect(() => {
    let timeout: ReturnType<typeof setTimeout>;
    if (isTyping) {
      setIsLookingAtEachOther(true);
      timeout = setTimeout(() => {
        setIsLookingAtEachOther(false);
      }, 800);
    } else {
      setIsLookingAtEachOther(false);
    }
    return () => clearTimeout(timeout);
  }, [isTyping]);

  // Purple peeking when password is visible
  useEffect(() => {
    let timeout: ReturnType<typeof setTimeout>;
    if (passwordLength > 0 && showPassword && !isPurplePeeking) {
      const interval = Math.random() * 3000 + 2000;
      timeout = setTimeout(() => {
        setIsPurplePeeking(true);
        setTimeout(() => {
          setIsPurplePeeking(false);
        }, 800);
      }, interval);
    } else if (passwordLength === 0 || !showPassword) {
      setIsPurplePeeking(false);
    }
    return () => clearTimeout(timeout);
  }, [passwordLength, showPassword, isPurplePeeking]);

  // Mount and unmount
  useEffect(() => {
    window.addEventListener('mousemove', handleMouseMove, { passive: true });
    window.addEventListener('resize', updateCharacterCenters, { passive: true });

    // Trigger entrance animation completion
    const entranceTimeout = setTimeout(() => {
      setHasEntered(true);
      updateCharacterCenters();
      rafIdRef.current = requestAnimationFrame(updatePositions);
    }, 1400);

    return () => {
      window.removeEventListener('mousemove', handleMouseMove);
      window.removeEventListener('resize', updateCharacterCenters);
      clearTimeout(entranceTimeout);
      if (rafIdRef.current) {
        cancelAnimationFrame(rafIdRef.current);
      }
    };
  }, [handleMouseMove, updateCharacterCenters, updatePositions]);

  return (
    <div className="animated-characters-container">
      {/* Confetti */}
      {showConfetti && (
        <div className="confetti-container">
          {confettiStyles.map((style, i) => (
            <div key={i} className="confetti-piece" style={style} />
          ))}
        </div>
      )}

      {/* Purple tall rectangle character - Back layer */}
      <div
        ref={purpleRef}
        className="character purple-character"
        style={{
          left: '70px',
          width: '180px',
          height: (isTyping || isHidingPassword) ? '440px' : '400px',
          backgroundColor: '#6C3FF5',
          borderRadius: '0',
          zIndex: 1,
          transform: hasEntered
            ? ((passwordLength > 0 && showPassword)
              ? 'skewX(0deg)'
              : (isTyping || isHidingPassword)
                ? `skewX(${purplePos.bodySkew - 12}deg) translateX(40px)`
                : `skewX(${purplePos.bodySkew}deg)`)
            : undefined,
        }}
      >
        {/* Eyes */}
        <div
          className="eyes"
          style={{
            left: (passwordLength > 0 && showPassword) ? '50px' : isLookingAtEachOther ? '85px' : `${75 + purplePos.faceX}px`,
            top: (passwordLength > 0 && showPassword) ? '20px' : isLookingAtEachOther ? '50px' : `${25 + purplePos.faceY}px`,
          }}
        >
          <EyeBall
            size={18}
            pupilSize={7}
            maxDistance={5}
            eyeColor="white"
            pupilColor="#2D2D2D"
            isBlinking={isPurpleBlinking}
            forceLookX={loginSuccess ? 0 : (passwordLength > 0 && showPassword) ? (isPurplePeeking ? 4 : -4) : isLookingAtEachOther ? 3 : undefined}
            forceLookY={loginSuccess ? successLookY : (passwordLength > 0 && showPassword) ? (isPurplePeeking ? 5 : -4) : isLookingAtEachOther ? 4 : undefined}
          />
          <EyeBall
            size={18}
            pupilSize={7}
            maxDistance={5}
            eyeColor="white"
            pupilColor="#2D2D2D"
            isBlinking={isPurpleBlinking}
            forceLookX={loginSuccess ? 0 : (passwordLength > 0 && showPassword) ? (isPurplePeeking ? 4 : -4) : isLookingAtEachOther ? 3 : undefined}
            forceLookY={loginSuccess ? successLookY : (passwordLength > 0 && showPassword) ? (isPurplePeeking ? 5 : -4) : isLookingAtEachOther ? 4 : undefined}
          />
        </div>
        {/* Mouth */}
        <div
          className={`purple-mouth-shape ${
            (isTyping || isHidingPassword) && !loginFailed && !loginSuccess ? 'purple-mouth-shape--typing' : ''
          } ${loginFailed ? 'purple-mouth-shape--sad' : ''} ${loginSuccess ? 'purple-mouth-shape--happy' : ''}`}
          style={{
            left: (passwordLength > 0 && showPassword) ? '72px' : isLookingAtEachOther ? '106px' : `${97 + purplePos.faceX}px`,
            top: (passwordLength > 0 && showPassword) ? '57px' : isLookingAtEachOther ? '82px' : `${57 + purplePos.faceY}px`,
            ['--counter-skew' as string]: (isTyping || isHidingPassword)
              ? `skewX(${-((purplePos.bodySkew || 0) - 12)}deg)`
              : 'skewX(0deg)',
          }}
        />
      </div>

      {/* Black tall rectangle character - Middle layer */}
      <div
        ref={blackRef}
        className="character black-character"
        style={{
          left: '240px',
          width: '120px',
          height: '310px',
          backgroundColor: '#2D2D2D',
          borderRadius: '0',
          zIndex: 2,
          transform: hasEntered
            ? ((passwordLength > 0 && showPassword)
              ? 'skewX(0deg)'
              : isLookingAtEachOther
                ? `skewX(${blackPos.bodySkew * 1.5 + 10}deg) translateX(20px)`
                : (isTyping || isHidingPassword)
                  ? `skewX(${blackPos.bodySkew * 1.5}deg)`
                  : `skewX(${blackPos.bodySkew}deg)`)
            : undefined,
        }}
      >
        {/* Eyes */}
        <div
          className="eyes"
          style={{
            left: (passwordLength > 0 && showPassword) ? '10px' : isLookingAtEachOther ? '32px' : `${26 + blackPos.faceX}px`,
            top: (passwordLength > 0 && showPassword) ? '28px' : isLookingAtEachOther ? '12px' : `${32 + blackPos.faceY}px`,
          }}
        >
          <EyeBall
            size={16}
            pupilSize={6}
            maxDistance={4}
            eyeColor="white"
            pupilColor="#2D2D2D"
            isBlinking={isBlackBlinking}
            isSad={loginFailed}
            sadRotate={-20}
            forceLookX={loginSuccess ? 0 : (passwordLength > 0 && showPassword) ? -4 : isLookingAtEachOther ? 0 : undefined}
            forceLookY={loginSuccess ? successLookY : (passwordLength > 0 && showPassword) ? -4 : isLookingAtEachOther ? -4 : undefined}
          />
          <EyeBall
            size={16}
            pupilSize={6}
            maxDistance={4}
            eyeColor="white"
            pupilColor="#2D2D2D"
            isBlinking={isBlackBlinking}
            isSad={loginFailed}
            sadRotate={20}
            forceLookX={loginSuccess ? 0 : (passwordLength > 0 && showPassword) ? -4 : isLookingAtEachOther ? 0 : undefined}
            forceLookY={loginSuccess ? successLookY : (passwordLength > 0 && showPassword) ? -4 : isLookingAtEachOther ? -4 : undefined}
          />
        </div>
      </div>

      {/* Orange semi-circle character - Front left */}
      <div
        ref={orangeRef}
        className="character orange-character"
        style={{
          left: '0px',
          width: '240px',
          height: '150px',
          zIndex: 3,
          backgroundColor: '#FF9B6B',
          borderRadius: '120px 120px 0 0',
          transform: hasEntered
            ? ((passwordLength > 0 && showPassword) ? 'skewX(0deg)' : `skewX(${orangePos.bodySkew}deg)`)
            : undefined,
        }}
      >
        {/* Eyes (just pupils) */}
        <div
          className="eyes"
          style={{
            left: (passwordLength > 0 && showPassword) ? '80px' : `${112 + orangePos.faceX}px`,
            top: (passwordLength > 0 && showPassword) ? '55px' : `${60 + orangePos.faceY}px`,
          }}
        >
          <Pupil
            size={12}
            maxDistance={5}
            pupilColor="#2D2D2D"
            isBlinking={isOrangeBlinking}
            forceLookX={loginSuccess ? 0 : (passwordLength > 0 && showPassword) ? -5 : undefined}
            forceLookY={loginSuccess ? successLookY : (passwordLength > 0 && showPassword) ? -4 : undefined}
          />
          <Pupil
            size={12}
            maxDistance={5}
            pupilColor="#2D2D2D"
            isBlinking={isOrangeBlinking}
            forceLookX={loginSuccess ? 0 : (passwordLength > 0 && showPassword) ? -5 : undefined}
            forceLookY={loginSuccess ? successLookY : (passwordLength > 0 && showPassword) ? -4 : undefined}
          />
        </div>
        {/* Mouth */}
        <div
          className={`orange-mouth-shape ${
            (isTyping || isHidingPassword) && !loginFailed && !loginSuccess ? 'orange-mouth-shape--typing' : ''
          } ${loginFailed ? 'orange-mouth-shape--sad' : ''} ${loginSuccess ? 'orange-mouth-shape--happy' : ''}`}
          style={{
            left: (passwordLength > 0 && showPassword) ? '94px' : `${126 + orangePos.faceX}px`,
            top: (passwordLength > 0 && showPassword) ? '87px' : `${92 + orangePos.faceY}px`,
          }}
        />
      </div>

      {/* Yellow tall rectangle character - Front right */}
      <div
        ref={yellowRef}
        className="character yellow-character"
        style={{
          left: '310px',
          width: '140px',
          height: '230px',
          backgroundColor: '#E8D754',
          borderRadius: '70px 70px 0 0',
          zIndex: 4,
          transform: hasEntered
            ? ((passwordLength > 0 && showPassword) ? 'skewX(0deg)' : `skewX(${yellowPos.bodySkew}deg)`)
            : undefined,
        }}
      >
        {/* Eyes (just pupils) */}
        <div
          className="eyes"
          style={{
            left: (passwordLength > 0 && showPassword) ? '20px' : `${52 + yellowPos.faceX}px`,
            top: (passwordLength > 0 && showPassword) ? '35px' : `${40 + yellowPos.faceY}px`,
          }}
        >
          <Pupil
            size={12}
            maxDistance={5}
            pupilColor="#2D2D2D"
            isBlinking={isYellowBlinking}
            forceLookX={loginSuccess ? 0 : (passwordLength > 0 && showPassword) ? -5 : undefined}
            forceLookY={loginSuccess ? successLookY : (passwordLength > 0 && showPassword) ? -4 : undefined}
          />
          <Pupil
            size={12}
            maxDistance={5}
            pupilColor="#2D2D2D"
            isBlinking={isYellowBlinking}
            forceLookX={loginSuccess ? 0 : (passwordLength > 0 && showPassword) ? -5 : undefined}
            forceLookY={loginSuccess ? successLookY : (passwordLength > 0 && showPassword) ? -4 : undefined}
          />
        </div>
        {/* Mouth */}
        <div
          className="yellow-mouth-wrapper"
          style={{
            left: (passwordLength > 0 && showPassword) ? '10px' : `${40 + yellowPos.faceX}px`,
            top: (passwordLength > 0 && showPassword) ? '88px' : `${88 + yellowPos.faceY}px`,
          }}
        >
          <svg width="80" height="20" viewBox="0 0 80 20">
            <path
              className={`yellow-mouth-path ${loginFailed ? 'yellow-mouth-path--wavy' : ''} ${loginSuccess ? 'yellow-mouth-path--happy' : ''}`}
              stroke="#2D2D2D"
              strokeWidth="3"
              fill="none"
              strokeLinecap="round"
            />
          </svg>
        </div>
      </div>
    </div>
  );
}

export default AnimatedCharacters;
