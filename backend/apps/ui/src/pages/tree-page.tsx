import { Alert, Layout, Result, Typography } from 'antd';
import { useParams, useSearchParams } from 'react-router';

import { parsePkPath } from '../api/pk';
import type { CallPK } from '../api/types';
import { parseTreeSearch } from '../url/search-params';

/**
 * Call Tree route (09 §3), opened in a new tab from a calls row with the
 * ts_ms / retention_class hints in the query string. Placeholder until 5.2
 * decodes and renders the tree.
 */
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

  return (
    <Layout style={{ minHeight: '100vh' }}>
      <Layout.Content style={{ padding: 24 }}>
        {parseError !== null || pk === null ? (
          <Alert type="error" message="Malformed call reference" description={parseError} showIcon />
        ) : (
          <Result
            status="info"
            title="Call tree lands with step 5.2"
            subTitle={
              <Typography.Text type="secondary">
                {pk.pod_namespace}/{pk.pod_service}/{pk.pod_name} · restart {pk.restart_time_ms} · ts{' '}
                {hints.tsMs ?? 'n/a'} · {hints.retentionClass ?? 'no retention hint'}
              </Typography.Text>
            }
          />
        )}
      </Layout.Content>
    </Layout>
  );
}
