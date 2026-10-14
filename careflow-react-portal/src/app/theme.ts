import { createTheme } from '@mui/material/styles';
export const makeTheme = (mode: 'light' | 'dark') =>
  createTheme({
    palette: {
      mode,
      primary: { main: '#3459e6' },
      secondary: { main: '#128970' },
      background: {
        default: mode === 'light' ? '#f6f8fc' : '#101927',
        paper: mode === 'light' ? '#ffffff' : '#172234',
      },
      text: {
        primary: mode === 'light' ? '#1b2940' : '#e7edf8',
        secondary: mode === 'light' ? '#5b6b82' : '#a4b1c7',
      },
      divider: mode === 'light' ? '#e9edf5' : '#2a374d',
      success: { main: '#128970' },
      warning: { main: '#bd7b16' },
      error: { main: '#d34a55' },
    },
    typography: {
      fontFamily: 'Inter, sans-serif',
      fontSize: 13,
      h1: { fontSize: '1.8rem', fontWeight: 650, letterSpacing: '-.045em' },
      h2: { fontSize: '1.2rem', fontWeight: 650, letterSpacing: '-.025em' },
      h3: { fontSize: '1rem', fontWeight: 650 },
      button: { textTransform: 'none', fontWeight: 550 },
      body1: { fontSize: '.85rem', lineHeight: 1.65 },
      body2: { fontSize: '.78rem', lineHeight: 1.6 },
    },
    shape: { borderRadius: 10 },
    components: {
      MuiButton: {
        defaultProps: { disableElevation: true },
        styleOverrides: {
          root: { borderRadius: 8, minHeight: 37, paddingInline: 15 },
          outlined: { borderColor: mode === 'light' ? '#dde3ee' : '#35445d' },
        },
      },
      MuiPaper: { defaultProps: { elevation: 0 }, styleOverrides: { root: { backgroundImage: 'none' } } },
      MuiCard: {
        styleOverrides: {
          root: {
            border: '1px solid',
            borderColor: mode === 'light' ? '#e7ecf4' : '#2a374d',
            boxShadow: '0 2px 4px rgba(21,40,79,.015)',
          },
        },
      },
      MuiTextField: { defaultProps: { size: 'small' } },
      MuiOutlinedInput: { styleOverrides: { root: { fontSize: '.82rem' } } },
      MuiChip: {
        styleOverrides: { root: { fontSize: '.7rem', fontWeight: 550, height: 25, borderRadius: 6 } },
      },
      MuiDialog: { styleOverrides: { paper: { borderRadius: 16 } } },
      MuiTableCell: {
        styleOverrides: {
          head: {
            fontSize: '.68rem',
            fontWeight: 600,
            color: mode === 'light' ? '#5b6b82' : '#a5b1c6',
            backgroundColor: mode === 'light' ? '#fbfcfe' : '#142030',
            paddingBlock: 12,
          },
          body: {
            fontSize: '.78rem',
            borderBottomColor: mode === 'light' ? '#edf0f6' : '#29364a',
            paddingBlock: 16,
          },
        },
      },
      MuiTooltip: { defaultProps: { arrow: true } },
      MuiSkeleton: { styleOverrides: { root: { borderRadius: 6 } } },
    },
  });
