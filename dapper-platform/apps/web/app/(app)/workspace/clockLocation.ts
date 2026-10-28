export type ClockFix = { latitude: number; longitude: number; accuracyMeters: number; capturedAt: string };

/* Clocking in needs a current location. Resolves null (never throws) when the browser cannot supply one, so the
   server, which knows whether the franchise requires location, makes the final decision and the user sees its reason. */
export function getClockInFix(): Promise<ClockFix | null> {
  return new Promise(resolve => {
    if (typeof navigator === 'undefined' || !('geolocation' in navigator)) return resolve(null);
    navigator.geolocation.getCurrentPosition(
      p => resolve({ latitude: p.coords.latitude, longitude: p.coords.longitude, accuracyMeters: p.coords.accuracy, capturedAt: new Date(p.timestamp || Date.now()).toISOString() }),
      () => resolve(null),
      { enableHighAccuracy: true, timeout: 15_000, maximumAge: 15_000 },
    );
  });
}
