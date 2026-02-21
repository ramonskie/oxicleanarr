import { useState } from 'react';
import { Film, Tv } from 'lucide-react';
import { buildPosterProxyURL, type PosterSize } from '../lib/imageUtils';

interface MediaPosterProps {
  /** Media item ID for the proxy URL */
  mediaId: string;
  /** Media type for fallback icon selection */
  mediaType: 'movie' | 'show';
  /** Whether a poster URL is available from the backend */
  hasPoster: boolean;
  /** Size preset controlling dimensions and image quality */
  size?: PosterSize;
  /** Additional CSS classes for the container */
  className?: string;
}

/** Pixel dimensions for each size preset (width x height at 2:3 poster ratio) */
const SIZE_DIMENSIONS: Record<PosterSize, { width: number; height: number; iconSize: number }> = {
  tiny:   { width: 32,  height: 48,  iconSize: 14 },
  small:  { width: 48,  height: 72,  iconSize: 18 },
  medium: { width: 120, height: 180, iconSize: 32 },
  large:  { width: 200, height: 300, iconSize: 48 },
};

/**
 * Reusable poster image component with loading skeleton, error fallback,
 * and type-appropriate icons for media without images.
 */
export function MediaPoster({
  mediaId,
  mediaType,
  hasPoster: hasPosterURL,
  size = 'small',
  className = '',
}: MediaPosterProps) {
  const [imageState, setImageState] = useState<'loading' | 'loaded' | 'error'>('loading');
  const dims = SIZE_DIMENSIONS[size];
  const FallbackIcon = mediaType === 'movie' ? Film : Tv;

  // No poster available — show fallback icon immediately
  if (!hasPosterURL) {
    return (
      <div
        className={`bg-[#333] rounded flex items-center justify-center flex-shrink-0 ${className}`}
        style={{ width: dims.width, height: dims.height }}
      >
        <FallbackIcon size={dims.iconSize} className="text-[#666]" />
      </div>
    );
  }

  const posterURL = buildPosterProxyURL(mediaId, size);

  return (
    <div
      className={`relative rounded overflow-hidden flex-shrink-0 ${className}`}
      style={{ width: dims.width, height: dims.height }}
    >
      {/* Loading skeleton (shown while image loads) */}
      {imageState === 'loading' && (
        <div className="absolute inset-0 bg-[#333] animate-pulse" />
      )}

      {/* Error fallback */}
      {imageState === 'error' && (
        <div className="absolute inset-0 bg-[#333] flex items-center justify-center">
          <FallbackIcon size={dims.iconSize} className="text-[#666]" />
        </div>
      )}

      {/* Actual image */}
      {imageState !== 'error' && (
        <img
          src={posterURL}
          alt=""
          loading="lazy"
          className={`w-full h-full object-cover transition-opacity duration-200 ${
            imageState === 'loaded' ? 'opacity-100' : 'opacity-0'
          }`}
          onLoad={() => setImageState('loaded')}
          onError={() => setImageState('error')}
        />
      )}
    </div>
  );
}
