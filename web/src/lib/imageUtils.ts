/** Size presets for poster images */
export type PosterSize = 'tiny' | 'small' | 'medium' | 'large';

interface ImageSizeConfig {
  maxWidth: number;
  quality: number;
}

const POSTER_SIZES: Record<PosterSize, ImageSizeConfig> = {
  tiny:   { maxWidth: 60,  quality: 70 },  // Table row thumbnails
  small:  { maxWidth: 120, quality: 80 },  // Small list items
  medium: { maxWidth: 300, quality: 85 },  // Grid cards
  large:  { maxWidth: 600, quality: 90 },  // Detail views
};

/**
 * Build the proxy URL for a media item's poster image.
 * Uses the backend proxy endpoint to avoid exposing Jellyfin API keys.
 */
export function buildPosterProxyURL(
  mediaId: string,
  size: PosterSize = 'small',
  imageType: 'Primary' | 'Backdrop' = 'Primary'
): string {
  const { maxWidth, quality } = POSTER_SIZES[size];
  const params = new URLSearchParams({
    type: imageType,
    maxWidth: maxWidth.toString(),
    quality: quality.toString(),
  });
  return `/api/media/${encodeURIComponent(mediaId)}/poster?${params}`;
}

/**
 * Check if a media item has a poster image available.
 */
export function hasPoster(item: { has_poster?: boolean }): boolean {
  return Boolean(item.has_poster);
}
