# CrocCoc

CrocCoc là ứng dụng desktop gửi và nhận tệp qua Croc, không cần thao tác bằng terminal. Mục tiêu của bản đầu là Windows 10/11 x64. [Kế hoạch CrocCoc trên Notion](https://kidounguyen.notion.site/CrocCoc-41cf424aba708182998fe301aaed3636) dùng một hạn chung là **25/12/2026**.

## Bản khởi tạo

Giao diện React có hai luồng Gửi và Nhận để xem thử bằng dữ liệu mẫu. Mock phát sự kiện theo cùng contract với backend và cho phép hủy phiên. Bản này không gửi, nhận hay ghi tệp thật. Backend có sẵn Wails bindings nhưng chủ động trả lỗi chưa tích hợp thay vì báo truyền thành công.

| Thành phần | Vị trí |
| --- | --- |
| Frontend | [`frontend/`](frontend) — React, TypeScript, Vite, mock và adapter Wails |
| Backend | [`backend/transfer/`](backend/transfer) — dịch vụ, kiểu dữ liệu và kiểm thử Go, không import Wails |
| Desktop host | `main.go`, `app.go`, `wails.json` — nhúng asset và nối UI với dịch vụ qua Wails IPC |
| Hồ sơ kỹ thuật | [Danh mục tài liệu](docs/README.md), [đặc tả yêu cầu](docs/requirements.md), [đặc tả API Wails IPC](docs/transfer-contract.md), [kiến trúc](docs/architecture.md), [kiểm thử](docs/verification.md), [ma trận truy vết](docs/traceability.md), [cấu hình](docs/configuration-management.md), [phát hành](docs/release.md) |

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

Chạy build frontend trước `go test` để Go có asset nhúng. Bản build Windows dự kiến nằm tại `build/bin/CrocCoc.exe`; Wails dùng icon mặc định ở bản khởi tạo. Chưa đóng gói installer hay binary Croc.

Phiên bản Croc chọn cho CC-02 là [v11.6.4](https://github.com/schollz/croc/releases/tag/v11.6.4). Task CC-02 sẽ thêm resolver và kiểm tra phiên bản. Việc kiểm thử truyền tệp thật, relay, hủy tiến trình và đối chiếu SHA-256 thuộc CC-08.
