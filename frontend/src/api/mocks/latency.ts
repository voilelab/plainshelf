/** Simulates backend latency so mock mode exercises the same loading states. */
export function delay<T>(value: T, ms = 240): Promise<T> {
  return new Promise((resolve) => {
    setTimeout(() => resolve(value), ms);
  });
}
