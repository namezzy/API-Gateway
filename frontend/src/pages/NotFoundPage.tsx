import { Box, Typography, Button } from '@mui/material';
import { Link } from 'react-router-dom';

export default function NotFoundPage() {
  return (
    <Box sx={{ p:5 }}>
      <Typography variant="h4" gutterBottom>404 Not Found</Typography>
      <Button component={Link} to="/" variant="contained">Go Home</Button>
    </Box>
  );
}
