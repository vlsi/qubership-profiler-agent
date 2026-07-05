import { Empty, Layout } from 'antd';
import { useSearchParams } from 'react-router';

import { parseCallsSearch } from '../url/search-params';

/** Discovery + Calls (09 §2). Placeholder until 5.1 lands the table. */
export function CallsPage() {
  const [searchParams] = useSearchParams();
  const search = parseCallsSearch(searchParams);

  return (
    <Layout.Content style={{ display: 'grid', placeItems: 'center', padding: 24 }}>
      <Empty
        description={
          search.fromMs === null
            ? 'Select a namespace or service and a period, then Apply.'
            : 'Calls table lands with step 5.1.'
        }
      />
    </Layout.Content>
  );
}
