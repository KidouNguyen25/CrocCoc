# ICD — Giao diện truyền tệp

| ID | Phiên bản | Trạng thái | Chủ trì |
| --- | --- | --- | --- |
| CC-ICD-001 | 0.1 | Bản nháp CC-01 | Phát, phối hợp Toàn và Minh |

Đây là contract dùng chung cho khung Wails và mock, chờ Phát xác nhận ở CC-01 trước khi các nhánh tính năng tích hợp. Không thay đổi tên hàm, event hay payload sau khi frontend bắt đầu tích hợp mà không thống nhất.

- Ranh giới Go và payload nằm tại [`backend/transfer/`](../backend/transfer). Wails giữ tên binding `main.App` và chuyển lời gọi vào `transfer.Service`.
- Ranh giới TypeScript nằm tại [`frontend/src/transfer.ts`](../frontend/src/transfer.ts). Giao diện làm việc qua `TransferClient`. Truy cập runtime Wails nằm riêng trong [adapter](../frontend/src/adapters/wailsTransfer.ts).
- Hiện tại, Go báo Croc chưa tích hợp và không tạo phiên thật. Mock phát cùng dạng event cho bản xem thử. Phần hành vi của phiên thật bên dưới là contract mục tiêu, chưa phải bằng chứng đã triển khai.

| Wails API | Đi vào | Trả về |
| --- | --- | --- |
| `GetCrocInfo()` | — | `available`, `version?`, `path?`, `message?` |
| `StartSend(paths)` | Mảng đường dẫn tệp hoặc thư mục | `transferID` hoặc lỗi khởi tạo |
| `StartReceive(code, outputDir)` | Mã kết nối và thư mục đích | `transferID` hoặc lỗi khởi tạo |
| `CancelTransfer(transferID)` | ID phiên cần hủy | `true` nếu yêu cầu hủy được chấp nhận, `false` nếu phiên không còn hoạt động hoặc sai ID, hoặc lỗi |

`transfer:update` gửi `transferID`, `state` và các trường tùy chọn `currentFile`, `bytesTransferred`, `bytesTotal`, `bytesPerSecond`, `progress`, `message`, `connectionCode`.

- Luồng thông thường đi từ `IDLE` → `PREPARING` → `WAITING_FOR_PEER` → `TRANSFERRING` → `COMPLETED`. Trạng thái đang chạy có thể kết thúc bằng `FAILED` hoặc `CANCELLED`. Backend chỉ cho một phiên hoạt động tại một thời điểm. `IDLE` là trạng thái UI khi chưa có phiên.
- Mỗi phiên có đúng một trạng thái kết thúc, không nhận cập nhật muộn sau khi kết thúc. Frontend đăng ký nhận event trước khi yêu cầu bắt đầu và đối chiếu `transferID` khi xử lý event.
- `progress` là phần trăm từ 0 đến 100. Bỏ trường này khi Croc không có số đo đáng tin cậy. Số byte và tốc độ cũng chỉ xuất hiện khi có dữ liệu.
- `connectionCode` chỉ đi từ backend đến giao diện người gửi để hiển thị và sao chép. Không nối mã vào `message`, không đưa vào console, telemetry, cấu hình hay lịch sử. Khi tích hợp, ưu tiên cơ chế biến môi trường mà bản Croc đã khóa hỗ trợ.
- CC-02/03 gọi Croc bằng đối số riêng qua runner có context, không dùng shell. Cần giải phóng tiến trình, reader và goroutine khi hủy hoặc thoát ứng dụng. Lỗi công khai phải có hướng xử lý và không kèm mã kết nối hoặc output thô.

## Bàn giao

| Đầu việc | Phụ trách | Phần còn lại sau init |
| --- | --- | --- |
| CC-01 | Phát | Xác nhận contract và quy ước phát event |
| CC-02, CC-03 | Phát | Resolver, runner, truyền thực, hủy và vòng đời |
| CC-04 đến CC-06 | Toàn | UI chính, hộp thoại chọn tệp/thư mục, sao chép mã, mở thư mục đích |
| CC-07, CC-08 | Minh | Đóng gói Croc và bản Windows x64, QA hai máy, relay và đối chiếu SHA-256 |

Tạo nhánh riêng theo mã ticket. PR nêu mã ticket, cách kiểm thử và minh chứng. Giữ phạm vi sở hữu `backend/transfer/` của Phát, `frontend/src/` của Toàn, và Wails/build/test tích hợp/tài liệu của Minh. Phối hợp trước khi sửa chéo. Khi thay đổi contract, rà lại [yêu cầu](requirements.md) và [ma trận truy vết](verification.md).
