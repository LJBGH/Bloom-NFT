import { useCallback, useRef, useState } from "react";
import type { ReactNode } from "react";
import {
  Button,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  Typography,
} from "@mui/material";

export type ConfirmOptions = {
  title: string;
  description: ReactNode;
  confirmText?: string;
  cancelText?: string;
};

/**
 * MUI 二次确认：返回 Promise，点为「确定」则 resolve(true)，关闭/取消为 false。
 * 在页面根节点旁渲染 `confirmDialog`。
 */
export function useConfirmDialog() {
  const [open, setOpen] = useState(false);
  const [options, setOptions] = useState<ConfirmOptions>({
    title: "",
    description: "",
  });
  const resolveRef = useRef<((v: boolean) => void) | null>(null);

  const requestConfirm = useCallback((opts: ConfirmOptions) => {
    return new Promise<boolean>((resolve) => {
      setOptions(opts);
      resolveRef.current = resolve;
      setOpen(true);
    });
  }, []);

  const finish = useCallback((confirmed: boolean) => {
    setOpen(false);
    const r = resolveRef.current;
    resolveRef.current = null;
    r?.(confirmed);
  }, []);

  const confirmDialog = (
    <Dialog
      open={open}
      onClose={() => finish(false)}
      maxWidth="xs"
      fullWidth
      slotProps={{
        paper: { sx: { borderRadius: 2 } },
      }}
    >
      <DialogTitle>{options.title}</DialogTitle>
      <DialogContent>
        {typeof options.description === "string" ? (
          <Typography
            variant="body2"
            color="text.secondary"
            sx={{ whiteSpace: "pre-wrap" }}
          >
            {options.description}
          </Typography>
        ) : (
          options.description
        )}
      </DialogContent>
      <DialogActions sx={{ px: 3, pb: 2 }}>
        <Button onClick={() => finish(false)} color="inherit">
          {options.cancelText ?? "取消"}
        </Button>
        <Button
          onClick={() => finish(true)}
          variant="contained"
          color="primary"
        >
          {options.confirmText ?? "确定"}
        </Button>
      </DialogActions>
    </Dialog>
  );

  return { requestConfirm, confirmDialog };
}
