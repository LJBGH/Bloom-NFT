import { createRoot } from "react-dom/client";
import {
  CssBaseline,
  ThemeProvider,
  createTheme,
  responsiveFontSizes,
} from "@mui/material";
import App from "./App";
import { Web3Provider } from "./web3/provider";

let theme = createTheme({
  palette: {
    mode: "dark",
    primary: {
      main: "#7c4dff",
    },
    background: {
      default: "#050816",
      paper: "#090b1f",
    },
  },
  shape: {
    borderRadius: 12,
  },
});
theme = responsiveFontSizes(theme);

createRoot(document.getElementById("root")!).render(
  <ThemeProvider theme={theme}>
    <CssBaseline />
    <Web3Provider>
      <App />
    </Web3Provider>
  </ThemeProvider>
);
