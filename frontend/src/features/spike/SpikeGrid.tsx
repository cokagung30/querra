import {
  columnResizingFeature,
  columnSizingFeature,
  createColumnHelper,
  tableFeatures,
  useTable,
} from '@tanstack/react-table'
import { useVirtualizer } from '@tanstack/react-virtual'
import { useEffect, useMemo, useSyncExternalStore, type RefObject } from 'react'

import type { DatasetInfo } from './api'
import type { Row, RowPageCache } from './pageCache'

const ROW_HEIGHT = 26
const HEADER_HEIGHT = 30
const NO_ROWS: Row[] = []

// TanStack Table only supplies the column model (sizes, resizing) here: rows
// live in Go and arrive page by page through the cache.
const features = tableFeatures({ columnSizingFeature, columnResizingFeature })
const columnHelper = createColumnHelper<typeof features, Row>()

interface SpikeGridProps {
  dataset: DatasetInfo
  cache: RowPageCache
  scrollRef: RefObject<HTMLDivElement | null>
}

export function SpikeGrid({ dataset, cache, scrollRef }: SpikeGridProps) {
  useSyncExternalStore(cache.subscribe, cache.getVersion)

  const columns = useMemo(
    () =>
      (dataset.columns ?? []).map((column) =>
        columnHelper.display({ id: column.name, header: column.name, size: 170, minSize: 60 }),
      ),
    [dataset.columns],
  )
  const table = useTable({ features, columns, data: NO_ROWS, columnResizeMode: 'onChange' })
  const headers = table.getHeaderGroups()[0]?.headers ?? []
  const sizes = headers.map((header) => header.getSize())

  const rowVirtualizer = useVirtualizer({
    count: dataset.rowCount,
    getScrollElement: () => scrollRef.current,
    estimateSize: () => ROW_HEIGHT,
    overscan: 12,
    paddingStart: HEADER_HEIGHT,
  })
  const columnVirtualizer = useVirtualizer({
    horizontal: true,
    count: sizes.length,
    getScrollElement: () => scrollRef.current,
    estimateSize: (index) => sizes[index],
    overscan: 3,
  })

  const sizeKey = sizes.join(',')
  useEffect(() => {
    columnVirtualizer.measure()
  }, [columnVirtualizer, sizeKey])

  const virtualRows = rowVirtualizer.getVirtualItems()
  const virtualColumns = columnVirtualizer.getVirtualItems()
  const firstRow = virtualRows[0]?.index ?? 0
  const lastRow = virtualRows.at(-1)?.index ?? 0
  useEffect(() => {
    cache.ensureRange(firstRow, lastRow)
  }, [cache, firstRow, lastRow])

  const totalWidth = columnVirtualizer.getTotalSize()

  return (
    <div
      ref={scrollRef}
      className="relative min-h-0 flex-1 overflow-auto rounded-md border font-mono text-xs"
    >
      <div
        style={{ position: 'relative', height: rowVirtualizer.getTotalSize(), width: totalWidth }}
      >
        <div
          className="sticky top-0 z-10 border-b bg-muted"
          style={{ height: HEADER_HEIGHT, width: totalWidth }}
        >
          {virtualColumns.map((virtualColumn) => {
            const header = headers[virtualColumn.index]
            return (
              <div
                key={virtualColumn.key}
                className="absolute top-0 flex h-full items-center truncate border-r px-2 font-semibold"
                style={{ left: virtualColumn.start, width: virtualColumn.size }}
              >
                {dataset.columns?.[virtualColumn.index]?.name}
                {header && (
                  <div
                    onMouseDown={header.getResizeHandler()}
                    onTouchStart={header.getResizeHandler()}
                    className="absolute top-0 right-0 h-full w-1.5 cursor-col-resize select-none hover:bg-primary/30"
                  />
                )}
              </div>
            )
          })}
        </div>

        {virtualRows.map((virtualRow) => {
          const row = cache.row(virtualRow.index)
          return (
            <div
              key={virtualRow.key}
              data-blank={row ? undefined : ''}
              className="absolute top-0 left-0 border-b"
              style={{
                height: ROW_HEIGHT,
                width: totalWidth,
                transform: `translateY(${virtualRow.start}px)`,
              }}
            >
              {row &&
                virtualColumns.map((virtualColumn) => (
                  <div
                    key={virtualColumn.key}
                    className="absolute top-0 truncate border-r px-2"
                    style={{
                      left: virtualColumn.start,
                      width: virtualColumn.size,
                      lineHeight: `${ROW_HEIGHT}px`,
                    }}
                  >
                    <Cell value={row[virtualColumn.index]} />
                  </div>
                ))}
            </div>
          )
        })}
      </div>
    </div>
  )
}

function Cell({ value }: { value: string | null | undefined }) {
  if (value == null) return <span className="text-muted-foreground italic">NULL</span>
  return value
}
