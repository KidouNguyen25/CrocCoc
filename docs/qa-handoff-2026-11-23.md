# CrocCoc — Xử lý vấn đề bàn giao (2026-11-23)

**Đầu vào:** `croccoc-handoff-issues.md`, baseline `main@bfb9b1f`. **Trạng thái:** đã sửa một số P2 và chuẩn bị quy trình đóng gói/QA, nhưng **chưa đủ bằng chứng nghiệm thu MVP/EXE**. Các thay đổi hiện nằm trong working tree (chưa commit/push).

## Đã thay đổi

1. **P2 worker thừa kế env:** `backend/transfer/worker_client.go` chỉ truyền `SystemRoot/WINDIR/TEMP/TMP/TMPDIR` (nếu có), không tự thừa kế proxy, biến cá nhân và thông tin đăng nhập của tiến trình host. Test worker dùng override tường minh, bổ sung ca xác nhận credential và HTTPS_PROXY không lọt sang tiến trình con.
2. **P2 Job Object:** tăng `TestWorkerJobObjectTerminatesProcessOnClose` để đợi `ping.exe` con thực sự chạy, mở handle cho child và xác minh cả `cmd.exe` lẫn tiến trình con đã kết thúc sau khi đóng Job Object.
3. **P2 reset UI khi mất tracking:** thay nút **Đặt lại màn hình** lúc phiên có thể còn chạy bằng **Yêu cầu hủy phiên**. Sau khi yêu cầu hủy, probe snapshot; không xóa tracker trước khi backend xác nhận trạng thái kết thúc. `handleReset` thêm guard không bỏ trạng thái running.
4. **P2 line-ending:** chuẩn hóa TS/TSX/MJS/HTML/CSS và `.gitattributes` sang LF trong `.gitattributes`.
5. **Chuẩn bị packaging/E2E:** lấy các artifact đã có từ `origin/feat/CC-07-packaging` (script build-release, fixtures SHA-256, runbook, main_worker_path_test); `main.go` tìm `bin/croccoc-engine.exe` cạnh binary trước khi báo unavailable, chỉ có override tường minh và không fallback PATH/cwd. Runbook được mở rộng ca Win+V/cloud, NTFS, relay, Windows 10/11 sạch, ký và SBOM; làm rõ `OpenReceiveDirectory` chưa có.

## Bằng chứng chạy

| Kiểm tra | Kết quả |
| --- | --- |
| `npm --prefix frontend test` | **PASS 22/22** |
| `npm --prefix frontend run build` | **PASS**, TypeScript + Vite |
| `node --check scripts/qa/fixtures.mjs` | **PASS** |
| Fixture `make ... --large-mb 0` | **PASS**: 4 tệp, 1 thư mục rỗng |
| Fixture `compare` cùng thư mục | **PASS** 4/4 tệp, 1/1 thư mục |
| Fixture `compare` với nội dung/đường dẫn khác | **FAIL đúng kỳ vọng** (exit 1, thiếu tệp) |
| `git diff --check` | **PASS** |
| `go version` / `go test ./backend/transfer` | **BLOCKED**: `go` không có trên PATH của CodeLocal |
| `wails doctor` | **BLOCKED**: `wails` không có trên PATH |
| `docker version` / `docker images` | **BLOCKED**: Docker Desktop Linux daemon không hoạt động |
| `gofmt` qua CodeLocal format | **BLOCKED**: `gofmt` không có trên PATH |
| Truyền relay 2 máy Windows; Win+V/cloud 2 máy; smoke Windows 10/11; build/ký EXE | **CHƯA CHẠY** |

## Chưa đóng (không tự đánh dấu PASS)

- **Bắt buộc trước khi coi Go/Windows patch hoàn tất:** chạy `gofmt`, `go test -count=1 ./...`, `go vet ./...`, test Job Object trên Windows x64 và smoke native Wails/worker. Máy có Go ở `E:\go` được mô tả trong QA trước, nhưng CodeLocal **từ chối truy cập đường dẫn ngoài workspace**, nên không lách chính sách quyền để chạy.
- **Release gates:** đóng gói nguồn worker với pin/hash, `wails build`, `govulncheck`, `npm audit`, SBOM/license, WebView2, ký Authenticode có timestamp và chạy E2E theo `docs/e2e-runbook.md` trên hai máy thật. Script hiện chỉ tạo **ứng viên chưa ký**, không đủ để tuyên bố public release.
- **P2 UI DOM:** chưa có React component/Playwright regression tự động cho luồng mất tracking; thay đổi UI mới chỉ vượt qua typecheck/build và helper test cũ.
- **Nits/post-MVP:** `CloseHandle` retry lease, tên `busy`, duplicate revision không xác định, Wails structured errors, mở thư mục nhận và shutdown bất đồng bộ vẫn là nợ kỹ thuật.

## Rà soát logic bổ sung (không chạy kiểm thử theo phân công)

Theo phân công ngày 2026-11-23, kiểm thử/QA do đội khác thực hiện. Đã đọc luồng coordinator, worker và UI; sửa thêm trong `backend/transfer/engine_service_windows.go`:

- **Race gửi thành công / hủy:** Worker có thể trả thành công cùng lúc `CancelTransfer` thắng khóa `CANCELLING`. Các bước `TRANSFERRING → FINALIZING → COMPLETED` sẽ bị từ chối, khiến coordinator mắc kẹt `BUSY` dù goroutine đã hoàn thành. Sau các bước thành công, nay có nhánh đưa `CANCELLING → CANCELLED`; nếu bất thường vẫn chưa terminal thì chuyển `FAILED`.
- **Dọn session:** `releaseSession` không còn xóa `sessions/sendCode` khi coordinator chưa terminal hoặc `cleanupPending`, bảo toàn đường điều khiển/hủy và chống mất tài nguyên theo dõi. Vẫn cho phép dọn khi receiver finalizer đã giải phóng terminal snapshot.
- **Shutdown pin worker:** `done` đóng không đồng nghĩa OS worker đã chết nếu cleanup timeout. `Shutdown` giữ khóa worker hash khi snapshot còn `CleanupPending`, tránh nhả executable pin quá sớm.

**Bàn giao QA:** đội kiểm thử cần rà lại các lịch hủy trùng lúc worker hoàn tất, worker cleanup timeout nhưng `done` đã đóng, nhận terminal snapshot đã được `FinalizeNativeReceive` release, và điều kiện `BUSY`/giải phóng pin. Chưa có bằng chứng chạy test cho ba thay đổi này; không tự công bố pass.

Chưa tạo commit/push; phần thay đổi logic đã lưu vào working tree và bàn giao kiểm thử cho đội QA.
