import { DownloadOutlined } from '@ant-design/icons';
import { Alert, Button, Empty, Layout, Result, Space, Spin, Table, Tabs, Tag, Typography } from 'antd';
import { useMemo, useState } from 'react';
import { useParams, useSearchParams } from 'react-router';

import { parsePkPath, pkToPath } from '../api/pk';
import type { CallPK } from '../api/types';
import { formatCount, formatDurationMs, formatTs } from '../calls/format';
import { summariseParams } from '../tree/params-summary';
import { TreeView } from '../tree/tree-view';
import { useTree } from '../tree/use-tree';
import { parseTreeSearch } from '../url/search-params';

// Call Tree route (09 §3): opened in a new tab from a calls row, carrying
// the §2.2 cold hints in the query string. Tabs: Call Tree · Hotspots ·
// Parameters (Database and Gantt are dropped).

export function TreePage() {
  const { pk: pkRaw } = useParams<{ pk: string }>();
  const [searchParams] = useSearchParams();
  const hints = parseTreeSearch(searchParams);

  let pk: CallPK | null = null;
  let parseError: string | null = null;
  try {
    pk = parsePkPath(pkRaw ?? '');
  } catch (e) {
    parseError = e instanceof Error ? e.message : String(e);
  }

  const { state, refetch } = useTree(pk, {
    tsMs: hints.tsMs ?? undefined,
    retentionClass: hints.retentionClass ?? undefined,
  });
  const [capped, setCapped] = useState(false);

  const model = state.kind === 'ready' ? state.model : null;
  const paramStats = useMemo(() => (model === null ? [] : summariseParams(model)), [model]);

  if (parseError !== null || pk === null) {
    return (
      <Layout style={{ minHeight: '100vh' }}>
        <Layout.Content style={{ padding: 24 }}>
          <Alert type="error" showIcon title="Malformed call reference" description={parseError} />
        </Layout.Content>
      </Layout>
    );
  }

  const traceHref = (() => {
    const sp = new URLSearchParams();
    if (hints.tsMs !== null) sp.set('ts_ms', String(hints.tsMs));
    if (hints.retentionClass !== null) sp.set('retention_class', hints.retentionClass);
    const qs = sp.toString();
    return `/api/v1/calls/${encodeURIComponent(pkToPath(pk))}/trace${qs === '' ? '' : `?${qs}`}`;
  })();

  return (
    <Layout style={{ height: '100vh' }}>
      <Layout.Header style={{ display: 'flex', alignItems: 'center', gap: 16, color: '#fff' }}>
        <Typography.Title level={5} style={{ margin: 0, color: '#fff', whiteSpace: 'nowrap' }}>
          {pk.pod_namespace} / {pk.pod_service} / {pk.pod_name}
        </Typography.Title>
        <Space>
          {hints.tsMs !== null ? <Tag>{formatTs(hints.tsMs)}</Tag> : null}
          {model !== null ? <Tag color="blue">{formatDurationMs(model.root.durationMs)}</Tag> : null}
          {hints.retentionClass !== null ? <Tag>{hints.retentionClass}</Tag> : null}
        </Space>
        <span style={{ flex: 1 }} />
        <Button size="small" icon={<DownloadOutlined />} href={traceHref} download>
          Raw trace
        </Button>
      </Layout.Header>
      <Layout.Content style={{ padding: '8px 16px', display: 'flex', flexDirection: 'column', minHeight: 0 }}>
        {state.kind === 'loading' ? (
          <div style={{ display: 'grid', placeItems: 'center', flex: 1 }}>
            <Spin description="Fetching and decoding the call tree." />
          </div>
        ) : state.kind === 'cold' ? (
          <Result
            status="info"
            title="This call is outside the hot window"
            subTitle={
              <>
                <Typography.Paragraph>
                  Reopen the call from its row in the calls list, so the ts_ms and retention_class hints travel
                  with it (02 §2.2).
                </Typography.Paragraph>
                <Typography.Text type="secondary">{state.detail}</Typography.Text>
              </>
            }
          />
        ) : state.kind === 'truncated' ? (
          <Result
            status="warning"
            title="The trace for this call was dropped under load — no tree to show"
            subTitle={state.detail}
          />
        ) : state.kind === 'error' ? (
          <Alert
            type="error"
            showIcon
            title="Loading the tree failed"
            description={state.message}
            action={
              <Button size="small" onClick={refetch}>
                Retry
              </Button>
            }
          />
        ) : model !== null ? (
          <>
            <Space orientation="vertical" style={{ width: '100%' }} size={4}>
              {model.hasUnresolvedParams ? (
                <Alert
                  type="warning"
                  showIcon
                  title="Some big parameters could not be resolved"
                  description="Their value segments were evicted before the seal; the affected groups are tagged 'unresolved' and shown as references, not values."
                />
              ) : null}
              {capped ? (
                <Alert
                  type="warning"
                  showIcon
                  title="Large tree — automatic expansion was limited"
                  description="Branches are still expandable by hand; the raw trace download carries full fidelity."
                />
              ) : null}
            </Space>
            <Tabs
              style={{ flex: 1, minHeight: 0 }}
              defaultActiveKey="tree"
              items={[
                {
                  key: 'tree',
                  label: 'Call Tree',
                  children: (
                    <div style={{ height: 'calc(100vh - 200px)' }}>
                      <TreeView model={model} onCapped={setCapped} />
                    </div>
                  ),
                },
                {
                  key: 'hotspots',
                  label: 'Hotspots',
                  children: <Empty description="Hotspots land with step 5.3." />,
                },
                {
                  key: 'params',
                  label: 'Parameters',
                  children: (
                    <Table
                      size="small"
                      rowKey={(r) => `${r.keyIdx}:${r.value}`}
                      dataSource={paramStats}
                      pagination={{ pageSize: 50, showSizeChanger: false }}
                      columns={[
                        {
                          title: 'Key',
                          width: 160,
                          render: (_, r) => <Tag>{model.paramKeys[r.keyIdx] ?? r.keyIdx}</Tag>,
                        },
                        {
                          title: 'Value',
                          render: (_, r) => (
                            <Typography.Text ellipsis style={{ maxWidth: 640 }} title={r.value}>
                              {r.value}
                              {r.unresolved ? <Tag color="orange" style={{ marginLeft: 8 }}>unresolved</Tag> : null}
                            </Typography.Text>
                          ),
                        },
                        {
                          title: 'Time',
                          width: 120,
                          align: 'right',
                          sorter: (a, b) => a.durationMs - b.durationMs,
                          render: (_, r) => formatDurationMs(r.durationMs),
                        },
                        {
                          title: 'Count',
                          width: 100,
                          align: 'right',
                          sorter: (a, b) => a.executions - b.executions,
                          render: (_, r) => formatCount(r.executions),
                        },
                        { title: 'Nodes', width: 90, align: 'right', render: (_, r) => formatCount(r.nodes) },
                      ]}
                    />
                  ),
                },
              ]}
            />
          </>
        ) : null}
      </Layout.Content>
    </Layout>
  );
}
