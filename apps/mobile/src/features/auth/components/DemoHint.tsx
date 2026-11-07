import { Notice } from '@/shared/ui/Rows';

/** Shown only in mock mode, so the owner knows what the fake backend accepts. */
export function DemoHint({ visible, title, body }: { visible: boolean; title: string; body: string }) {
  if (!visible) return null;
  return <Notice tone="info" icon="flask-outline" title={title} body={body} />;
}
