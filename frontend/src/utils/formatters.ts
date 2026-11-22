import type { TransferState } from '../transfer.ts'

const DECIMAL_UINT_RE = /^\d+$/

/**
 * Compute exact transfer percentage from decimal byte strings or numbers using BigInt.
 * Returns undefined when total is missing, invalid, or zero (indeterminate progress).
 */
export function computeProgressPercent(
  bytesTransferred?: number | string,
  bytesTotal?: number | string,
): number | undefined {
  if (bytesTransferred === undefined || bytesTotal === undefined) return undefined
  const tStr = typeof bytesTransferred === 'number'
    ? (Number.isInteger(bytesTransferred) && bytesTransferred >= 0 ? String(bytesTransferred) : '')
    : bytesTransferred
  const totalStr = typeof bytesTotal === 'number'
    ? (Number.isInteger(bytesTotal) && bytesTotal >= 0 ? String(bytesTotal) : '')
    : bytesTotal
  if (!DECIMAL_UINT_RE.test(tStr) || !DECIMAL_UINT_RE.test(totalStr)) return undefined
  const transferred = BigInt(tStr)
  const total = BigInt(totalStr)
  if (total <= 0n || transferred > total) return undefined
  const basisPoints = (transferred * 10000n) / total
  return Number(basisPoints) / 100
}

/**
 * Format bytes into human readable format (B, KB, MB, GB, TB).
 */
export function formatBytes(bytes?: number | string): string {
  if (bytes === undefined || bytes === null) return 'Chưa rõ dung lượng'
  const num = typeof bytes === 'string' ? Number(bytes) : bytes
  if (isNaN(num) || num < 0) return '0 B'
  if (num === 0) return '0 B'

  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  const k = 1024
  const i = Math.floor(Math.log(num) / Math.log(k))
  const index = Math.min(i, units.length - 1)
  const val = num / Math.pow(k, index)

  const formatted = val < 10 && index > 0 && val % 1 !== 0
    ? val.toFixed(1)
    : Math.round(val).toString()

  return `${formatted} ${units[index]}`
}

/**
 * Format transfer speed (e.g., 2.5 MB/s).
 */
export function formatSpeed(bytesPerSecond?: number | string): string {
  if (bytesPerSecond === undefined || bytesPerSecond === null) return ''
  const num = typeof bytesPerSecond === 'string' ? Number(bytesPerSecond) : bytesPerSecond
  if (isNaN(num) || num <= 0) return '0 B/s'

  return `${formatBytes(num)}/s`
}

/**
 * Format remaining seconds into e.g., "12s", "1m 30s", "2h 15m".
 */
export function formatDuration(seconds?: number): string {
  if (seconds === undefined || isNaN(seconds) || seconds < 0 || !isFinite(seconds)) return ''
  const sec = Math.round(seconds)
  if (sec < 60) return `${sec}s`
  const minutes = Math.floor(sec / 60)
  const remainingSec = sec % 60
  if (minutes < 60) {
    return remainingSec > 0 ? `${minutes}m ${remainingSec}s` : `${minutes}m`
  }
  const hours = Math.floor(minutes / 60)
  const remainingMin = minutes % 60
  return remainingMin > 0 ? `${hours}h ${remainingMin}m` : `${hours}h`
}

/**
 * Calculate estimated time remaining (ETA).
 */
export function calculateETA(
  bytesTransferred?: number | string,
  bytesTotal?: number | string,
  bytesPerSecond?: number,
): string {
  const transferred = typeof bytesTransferred === 'string' ? Number(bytesTransferred) : bytesTransferred
  const total = typeof bytesTotal === 'string' ? Number(bytesTotal) : bytesTotal
  if (
    transferred === undefined ||
    total === undefined ||
    isNaN(transferred) ||
    isNaN(total) ||
    bytesPerSecond === undefined ||
    bytesPerSecond <= 0 ||
    transferred >= total
  ) {
    return ''
  }
  const remainingBytes = total - transferred
  const seconds = remainingBytes / bytesPerSecond
  return formatDuration(seconds)
}

export const STATE_LABELS: Record<TransferState, string> = {
  IDLE: 'Sẵn sàng',
  PREPARING: 'Đang chuẩn bị',
  WAITING_FOR_PEER: 'Đang chờ kết nối',
  AWAITING_ACCEPTANCE: 'Chờ người nhận xác nhận',
  TRANSFERRING: 'Đang truyền dữ liệu',
  FINALIZING: 'Đang hoàn tất',
  CANCELLING: 'Đang hủy...',
  COMPLETED: 'Truyền hoàn tất',
  FAILED: 'Truyền thất bại',
  CANCELLED: 'Đã hủy',
}

export const STATE_DESCRIPTIONS: Record<TransferState, string> = {
  IDLE: 'Chọn tệp để gửi hoặc nhập mã để nhận.',
  PREPARING: 'Đang khởi tạo phiên và kiểm tra dữ liệu...',
  WAITING_FOR_PEER: 'Đang chờ thiết bị bên kia kết nối...',
  AWAITING_ACCEPTANCE: 'Đã nhận danh sách tệp, chờ người nhận đồng ý...',
  TRANSFERRING: 'Dữ liệu đang được truyền mã hóa qua Croc...',
  FINALIZING: 'Đang xác minh toàn vẹn và công bố tệp vào thư mục đích...',
  CANCELLING: 'Đang dọn dẹp và kết thúc tiến trình...',
  COMPLETED: 'Tệp đã được truyền thành công và xác thực toàn vẹn.',
  FAILED: 'Đã xảy ra lỗi trong quá trình truyền tệp.',
  CANCELLED: 'Phiên truyền tệp đã được người dùng chủ động hủy.',
}
