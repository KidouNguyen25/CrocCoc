# AD — Mô tả kiến trúc CrocCoc

| ID | Phiên bản | Trạng thái | Phạm vi |
| --- | --- | --- | --- |
| CC-AD-001 | 0.1 | Kiến trúc khởi tạo | Ứng dụng desktop CrocCoc trên Windows x64 |

Tài liệu vận dụng [ISO/IEC/IEEE 42010:2022](https://www.iso.org/standard/74393.html) để ghi rõ bên liên quan, mối quan tâm, góc nhìn, quan hệ giữa các góc nhìn và lý do thiết kế. Đây là hồ sơ nội bộ, không phải tuyên bố chứng nhận tuân thủ.

## Bên liên quan và góc nhìn

| Bên liên quan | Mối quan tâm | Góc nhìn |
| --- | --- | --- |
| Người gửi, người nhận | Mã kết nối, trạng thái, nơi lưu, lỗi và hủy | Cấu trúc, luồng chạy |
| Toàn, frontend và review | Phát triển UI độc lập, không phụ thuộc Croc hay Go để chạy mock | Cấu trúc |
| Phát, backend | Lệnh CLI an toàn, xử lý lỗi và vòng đời tiến trình | Cấu trúc, luồng chạy |
| Minh, tích hợp và QA | Nhúng asset, WebView2, phiên bản CLI, build Windows và tái lập test | Triển khai |

## Góc nhìn cấu trúc

| Thành phần | Mã nguồn | Phụ thuộc và trách nhiệm |
| --- | --- | --- |
| FE — Web UI | `frontend/src/` | React hiển thị trạng thái. Gọi qua `TransferClient`; mock chạy không cần runtime desktop. |
| FE-IPC — Adapter TypeScript | `frontend/src/adapters/wailsTransfer.ts` | Ánh xạ `TransferClient` vào Wails IPC và chuyển event. Không giữ trạng thái nghiệp vụ. |
| HOST — Desktop host và bridge | `main.go`, `app.go`, `wails.json` | Khởi động Wails, nhúng `frontend/dist/`, nối binding `main.App` với `transfer.Service`. Không điều phối truyền tệp. |
| BE — Dịch vụ Go | `backend/transfer/` | Sở hữu contract và hành vi truyền tệp. Không import Wails hay giao diện. Hiện dùng dịch vụ trả trạng thái chưa tích hợp. |
| CLI — Tiến trình Croc | Ngoài repo | Dự kiến chạy từ backend qua runner CC-02/03. Không có kết nối CLI ở bản khởi tạo. |

## Góc nhìn luồng chạy

- Bản xem thử hiện tại dùng **FE → mock** để phát các trạng thái mẫu. Trên desktop, FE chỉ gọi trạng thái khả dụng qua **FE-IPC → HOST → BE**. Backend không phát event phiên thật ở bản này.
- Luồng mục tiêu dùng **FE → FE-IPC → HOST → BE → CLI**. Backend sở hữu vòng đời, chuyển cập nhật qua HOST thành event `transfer:update` cho FE-IPC và bảo đảm mỗi phiên chỉ có một kết quả cuối. Tên phương thức, trường dữ liệu và event được ghi trong [ICD](transfer-contract.md).
- Ranh giới bí mật nằm giữa phần nhập/hiển thị mã ở FE, IPC nội bộ và tiến trình Croc. Mã không đi qua log hay lưu trữ lâu dài. Backend không gọi shell với dữ liệu người dùng.

## Góc nhìn triển khai

Frontend build thành tài nguyên tĩnh và được nhúng vào `CrocCoc.exe`. Tiến trình ứng dụng chạy Wails và dịch vụ Go, còn WebView2 hiển thị giao diện. Khi tích hợp thật, Croc chạy ở tiến trình con do backend quản lý và phụ trách kết nối mạng. Không có HTTP API server, cơ sở dữ liệu hoặc dịch vụ relay riêng của CrocCoc. Bản phát triển Vite có thể chạy độc lập trên trình duyệt với mock.

## Quyết định và tính nhất quán

| Mã | Quyết định | Lý do và hệ quả |
| --- | --- | --- |
| ADR-01 | Wails IPC, không dựng HTTP backend. | Mục tiêu là desktop cục bộ. Một gói ứng dụng đơn giản hơn, nhưng tích hợp phụ thuộc WebView2 và lifecycle Wails. |
| ADR-02 | Giữ binding `main.App`, tiêm `transfer.Service` vào bridge. | Không đổi giao diện Wails khi di chuyển logic. Backend chạy test độc lập, bridge test forwarding và adapter test ánh xạ hợp đồng. |
| ADR-03 | Dùng Croc CLI thay vì viết lại giao thức. | Giảm phạm vi. CC-02/03 phải khóa binary, quản lý vòng đời và kiểm tra các trường hợp lỗi của CLI. |

Mô hình cấu trúc FE-IPC/HOST áp dụng đúng mapping trong ICD và được kiểm bởi `CC-T-02`. BE chạy độc lập theo `CC-T-01`; HOST nhúng artifact của FE theo `CC-T-03`. Phạm vi bản đầu và các ca chưa chạy nằm trong [hồ sơ kiểm thử](verification.md).
