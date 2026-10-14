import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { ThemeProvider } from '@mui/material';
import { describe, expect, it, vi } from 'vitest';
import { makeTheme } from '../app/theme';
import { ConfirmDialog, StatusBadge } from './components';
describe('accessible operational controls', () => {
  it('requires explicit confirmation before executing sensitive work', async () => {
    const confirm = vi.fn();
    render(
      <ThemeProvider theme={makeTheme('light')}>
        <ConfirmDialog
          open
          title="Cancel appointment?"
          description="Release the confirmed reservation."
          onClose={() => {}}
          onConfirm={confirm}
          confirm="Cancel appointment"
        />
      </ThemeProvider>,
    );
    expect(confirm).not.toHaveBeenCalled();
    expect(screen.getByRole('dialog')).toHaveAccessibleName('Cancel appointment?');
    await userEvent.click(screen.getByRole('button', { name: 'Cancel appointment', exact: true }));
    expect(confirm).toHaveBeenCalledOnce();
  });
  it('conveys status through readable text as well as color', () => {
    render(<StatusBadge status="awaiting_approvals" />);
    expect(screen.getByText('Pending approval')).toBeInTheDocument();
  });
});
