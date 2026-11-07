import { PostgreSQL, sql } from '@codemirror/lang-sql'
import { basicSetup, EditorView } from 'codemirror'
import { useEffect, useRef } from 'react'

const SAMPLE_QUERY = `-- Autocomplete knows the querra_test schema: try typing "sales." or "o."
SELECT o.id, c.name, sum(oi.quantity * p.price) AS total
FROM sales.orders o
JOIN sales.customers c ON c.id = o.customer_id
JOIN sales.order_items oi ON oi.order_id = o.id
JOIN sales.products p ON p.id = oi.product_id
GROUP BY o.id, c.name;
`

const SCHEMA = {
  sales: {
    customers: ['id', 'name', 'email'],
    products: ['id', 'sku', 'name', 'price'],
    orders: ['id', 'customer_id', 'ordered_at', 'status'],
    order_items: ['order_id', 'product_id', 'quantity'],
  },
  public: {
    big_table: ['id', 'name', 'category', 'amount', 'is_active', 'created_at'],
    no_pk: ['name', 'value'],
  },
}

// Loaded lazily (React.lazy) so CodeMirror stays out of the startup bundle.
export default function SqlEditor() {
  const hostRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    if (!hostRef.current) return
    const view = new EditorView({
      doc: SAMPLE_QUERY,
      parent: hostRef.current,
      extensions: [
        basicSetup,
        sql({ dialect: PostgreSQL, schema: SCHEMA, defaultSchema: 'public' }),
      ],
    })
    return () => view.destroy()
  }, [])

  return <div ref={hostRef} className="h-48 overflow-auto rounded-md border text-sm" />
}
