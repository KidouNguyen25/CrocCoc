# CrocCoc

CrocCoc là ứng dụng desktop gửi và nhận tệp qua Croc, không cần thao tác bằng terminal. Mục tiêu của bản đầu là Windows 10/11 x64. Mốc **25/12/2026** là tham chiếu lịch sử của hồ sơ init. [Kế hoạch CrocCoc trên Notion](https://kidounguyen.notion.site/CrocCoc-41cf424aba708182998fe301aaed3636) là liên kết tham khảo.

## Hiện trạng tích hợp

Giao diện React có hai luồng Gửi và Nhận, tự dùng Wails IPC trên desktop và fallback sang mock khi xem trước trên trình duyệt. Backend Windows (`backend/transfer/`) đã tích hợp engine helper Croc v11.6.4 pin SHA-256 (`backend/engine-spike/`), staging NTFS riêng có sổ sở hữu, publish no-replace theo handle, Job Object kill-on-close, `SingleInstanceLock`, `OnBeforeClose`/`OnShutdown`, và `CopySendCode` bảo vệ clipboard Win32. Host hiện xuất các binding: `GetCrocInfo`, `StartSend`, `StartReceive`, `CancelTransfer`, `DecideReceive`, `GetTransferSnapshot`, `SelectPaths`, `CopySendCode` (khớp [ICD 0.6](docs/transfer-contract.md); `OpenReceiveDirectory` còn mở). Khi thiếu cấu hình worker/relay (`CROCCOC_WORKER_PATH`, `CROCCOC_RELAY_ADDRESS`, `CROCCOC_RELAY_PASSWORD`), ứng dụng fail closed ở trạng thái chưa khả dụng.

| Thành phần | Vị trí |
| --- | --- |
| Frontend | [`frontend/`](frontend) — React, TypeScript, Vite, mock và adapter Wails |
| Backend | [`backend/transfer/`](backend/transfer) — dịch vụ, kiểu dữ liệu và kiểm thử Go, không import Wails |
| Desktop host | `main.go`, `app.go`, `wails.json` — nhúng asset và nối UI với dịch vụ qua Wails IPC |
| Hồ sơ kỹ thuật | [Danh mục tài liệu](docs/README.md), [đặc tả yêu cầu](docs/requirements.md), [đặc tả API Wails IPC](docs/transfer-contract.md), [kiến trúc](docs/architecture.md), [mô hình đe dọa](docs/threat-model.md), [kiểm thử](docs/verification.md), [ma trận truy vết](docs/traceability.md), [cấu hình](docs/configuration-management.md), [phát hành](docs/release.md) |

CrocCoc là ứng dụng desktop. Frontend và backend có ranh giới mã nguồn và kiểm thử riêng, không cần triển khai một API server.

## Chạy trên Windows

Cài Go 1.26+, Node.js 24+ và Microsoft Edge WebView2 Runtime. Dùng **Wails v2.17.1** cho cả thư viện lẫn CLI. Thêm thư mục `$(go env GOPATH)\bin` vào `PATH` của máy phát triển nếu chưa có.

```powershell
go install github.com/wailsapp/wails/v2/cmd/wails@v2.17.1
wails doctor
wails dev
```

Chỉ chạy giao diện mô phỏng trong trình duyệt, không cần Go hay Wails.

```powershell
npm --prefix frontend ci
npm --prefix frontend run dev
```

## Kiểm tra và build

Kiểm thử riêng backend không cần Node, asset frontend, WebView hay Croc.

```powershell
go test ./backend/...
```

Để kiểm tra toàn ứng dụng từ gốc repo.

```powershell
npm --prefix frontend ci
npm --prefix frontend test
npm --prefix frontend run build
go test ./...
wails build -platform windows/amd64
```

Chạy build frontend trước `go test` để Go có asset nhúng. `wails build` có thể build lại frontend theo `wails.json`. Bản build Windows dự kiến nằm tại `build/bin/CrocCoc.exe`; Wails dùng icon mặc định ở bản khởi tạo. Chưa đóng gói installer hay binary Croc.

[Croc v11.6.4](https://github.com/schollz/croc/releases/tag/v11.6.4) là pin để làm spike, không phải engine đã tích hợp. [Cổng tích hợp](docs/architecture.md#adr-03--chọn-engine-sau-spike) yêu cầu metadata có cấu trúc, xác nhận trước khi tải, ghi an toàn và hủy có hạn. Xem [vạch cắt MVP](docs/requirements.md#vạch-cắt-mvp) và [ca xác minh](docs/verification.md).
