import { Box, Button, Card, InputAdornment, MenuItem, Stack, TextField, Typography } from '@mui/material';
import { DataGrid, type GridColDef } from '@mui/x-data-grid';
import { Download, Search } from 'lucide-react';
import { useMemo, useState } from 'react';
import type { Entity } from './types';
import { label } from './format';
import { useSession } from '../stores/session';

export function DataTable({
  rows,
  columns,
  onRow,
  loading = false,
  hasMore = false,
  loadMore,
  statuses,
  empty = 'No matching records',
  name = 'records',
  exportable = false,
}: {
  rows: Entity[];
  columns: GridColDef[];
  onRow?: (row: Entity) => void;
  loading?: boolean;
  hasMore?: boolean;
  loadMore?: () => void;
  statuses?: string[];
  empty?: string;
  name?: string;
  exportable?: boolean;
}) {
  const [search, setSearch] = useState('');
  const [status, setStatus] = useState('all');
  const masked = useSession((s) => s.maskPhi);
  const visible = useMemo(
    () =>
      rows.filter(
        (row) =>
          (status === 'all' || row.status === status) &&
          (search.trim() === '' ||
            Object.entries(row)
              .filter(([key]) => !masked || !['name', 'birth_date', 'external_id'].includes(key))
              .some(
                ([, value]) =>
                  typeof value === 'string' && value.toLowerCase().includes(search.toLowerCase()),
              )),
      ),
    [rows, search, status, masked],
  );
  const download = () => {
    const header = columns.filter((c) => c.field !== 'actions').map((c) => c.field);
    const cell = (value: unknown) =>
      `"${String(value ?? '')
        .replace(/^[=+@-]/, "'$&")
        .replaceAll('"', '""')}"`;
    const csv = [
      header.map(cell).join(','),
      ...visible.map((r) => header.map((k) => cell(r[k])).join(',')),
    ].join('\n');
    const url = URL.createObjectURL(new Blob([csv], { type: 'text/csv;charset=utf-8' }));
    const a = document.createElement('a');
    a.href = url;
    a.download = `careflow-${name}.csv`;
    a.click();
    URL.revokeObjectURL(url);
  };
  return (
    <Card>
      <Stack
        className="table-toolbar"
        direction={{ xs: 'column', sm: 'row' }}
        justifyContent="space-between"
        gap={1.5}
      >
        <Stack direction="row" gap={1.3}>
          <TextField
            placeholder={`Search ${name}…`}
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            slotProps={{
              input: {
                startAdornment: (
                  <InputAdornment position="start">
                    <Search size={16} />
                  </InputAdornment>
                ),
              },
              htmlInput: { 'aria-label': `Search ${name}` },
            }}
            sx={{ width: { xs: '100%', sm: 280 } }}
          />
          {statuses && (
            <TextField
              select
              value={status}
              onChange={(e) => setStatus(e.target.value)}
              sx={{ minWidth: 155 }}
              slotProps={{ htmlInput: { 'aria-label': 'Filter by status' } }}
            >
              <MenuItem value="all">All statuses</MenuItem>
              {statuses.map((s) => (
                <MenuItem key={s} value={s}>
                  {label(s)}
                </MenuItem>
              ))}
            </TextField>
          )}
        </Stack>
        {exportable && (
          <Button variant="outlined" startIcon={<Download size={15} />} onClick={download}>
            Export
          </Button>
        )}
      </Stack>
      <Box sx={{ height: Math.max(280, Math.min(650, visible.length * 64 + 130)), width: '100%' }}>
        <DataGrid
          rows={visible}
          columns={columns}
          loading={loading}
          onRowClick={onRow ? (params) => onRow(params.row) : undefined}
          disableRowSelectionOnClick
          rowHeight={64}
          columnHeaderHeight={44}
          pageSizeOptions={[10, 25, 50]}
          initialState={{ pagination: { paginationModel: { pageSize: 10, page: 0 } } }}
          slots={{
            noRowsOverlay: () => (
              <Box
                sx={{ display: 'grid', placeItems: 'center', height: '100%', p: 3, color: 'text.secondary' }}
              >
                {empty}
              </Box>
            ),
          }}
          sx={{
            border: 0,
            '& .MuiDataGrid-columnHeaders': { fontSize: 11, fontWeight: 600, color: 'text.secondary' },
            '& .MuiDataGrid-cell': {
              display: 'flex',
              alignItems: 'center',
              borderColor: 'divider',
              fontSize: 12,
            },
            '& .MuiDataGrid-row': { cursor: onRow ? 'pointer' : 'default' },
            '& .MuiDataGrid-cell:focus-within': { outline: '2px solid #3459e6', outlineOffset: -2 },
            '& .MuiDataGrid-footerContainer': { borderColor: 'divider' },
          }}
        />
      </Box>
      <Stack
        direction="row"
        alignItems="center"
        justifyContent="space-between"
        sx={{ px: 2.5, py: 1.3, borderTop: 1, borderColor: 'divider' }}
      >
        <Typography variant="caption" color="text.secondary">
          {visible.length} loaded {name}
          {hasMore ? ' · More records available' : ''}
        </Typography>
        {hasMore && (
          <Button size="small" onClick={loadMore} disabled={loading}>
            Load more records
          </Button>
        )}
      </Stack>
    </Card>
  );
}
