import { Empty, Layout } from 'antd';

/** Pods Info (09 §4). Placeholder until 5.1. */
export function PodsPage() {
  return (
    <Layout.Content style={{ display: 'grid', placeItems: 'center', padding: 24 }}>
      <Empty description="Pod-restart listing lands with step 5.1." />
    </Layout.Content>
  );
}
