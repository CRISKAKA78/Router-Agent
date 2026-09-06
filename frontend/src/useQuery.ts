import { useEffect, useRef, useState } from "react";
import type { Connection } from "./connection";
import { describe } from "./api";
// One worker per mounted query. Changes during an in-flight read merge into one
// follow-up read; the old result cannot overwrite a newer selection/connection.
export function useQuery<T>(
  connection: Connection | null,
  key: string,
  revision: number,
  read: (c: Connection) => Promise<T>,
  onError: (s: string) => void,
): T | null {
  const [data, setData] = useState<T | null>(null);
  const latest = useRef({ connection, key, read, onError, epoch: 0 });
  latest.current = {
    connection,
    key,
    read,
    onError,
    epoch: latest.current.epoch,
  };
  const worker = useRef<Promise<void> | null>(null),
    dirty = useRef(false),
    mounted = useRef(true);
  useEffect(() => {
    mounted.current = true;
    return () => {
      mounted.current = false;
      latest.current.epoch++;
    };
  }, []);
  useEffect(() => {
    setData(null);
  }, [connection, key]);
  useEffect(() => {
    latest.current.epoch++;
    dirty.current = true;
    if (!worker.current) {
      worker.current = Promise.resolve().then(async () => {
        try {
          while (dirty.current && mounted.current) {
            dirty.current = false;
            const request = latest.current;
            if (!request.connection || !request.key) continue;
            try {
              const value = await request.connection.track(() =>
                request.read(request.connection!),
              );
              if (mounted.current && request.epoch === latest.current.epoch)
                setData(value);
            } catch (e) {
              if (
                mounted.current &&
                request.epoch === latest.current.epoch &&
                !request.connection.lifetime.signal.aborted
              )
                request.onError(describe(e));
            }
          }
        } finally {
          worker.current = null;
        }
      });
    }
  }, [connection, key, revision]);
  return data;
}
