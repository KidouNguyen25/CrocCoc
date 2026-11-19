# API / ICD — Đặc tả giao diện truyền tệp CrocCoc

| Mã | Phiên bản | Ngày | Trạng thái | Mã nguồn đối chiếu |
| --- | --- | --- | --- | --- |
| CC-ICD-001 | 0.2 | 2026-11-19 | Dự thảo CC-01 | `610b622` |

## 1. Ranh giới và phạm vi

Đây là **API nội bộ Wails IPC và sự kiện**, không phải REST API. Không có URL gốc, endpoint HTTP, mã trạng thái HTTP hay cơ chế đăng nhập cho API này. WebView gọi binding `window.go.main.App` qua [adapter TypeScript](../frontend/src/adapters/wailsTransfer.ts). [Bridge Go](../app.go) chuyển lời gọi đến [`transfer.Service`](../backend/transfer/service.go). Hợp đồng dữ liệu được mô tả bằng [kiểu Go](../backend/transfer/contract.go) và [kiểu TypeScript](../frontend/src/transfer.ts). [Wails Events](https://wails.io/docs/v2.15.0/reference/runtime/events/) là tài liệu cơ chế sự kiện họ Wails v2, không thay phiên bản phụ thuộc trong repo.

**Bản init.** Desktop chỉ dùng `GetCrocInfo` cho nhãn khả dụng. Ba lệnh còn lại đã có binding nhưng dịch vụ trả `ErrNotIntegrated` và không phát sự kiện phiên thật. [Client mock](../frontend/src/mockTransfer.ts) mô phỏng cùng kiểu dữ liệu nhưng không gọi Croc, không đọc hoặc ghi tệp. Các quy tắc ghi **Mục tiêu** chưa phải cam kết về hành vi đang chạy.

## 2. Lời gọi

| Mã | Binding Wails → phương thức UI | Đầu vào | Kết quả bản init | Mục tiêu |
| --- | --- | --- | --- | --- |
| CC-API-01 | `GetCrocInfo()` → `getCrocInfo()` | Không có | Promise resolve `{ available: false, message: … }`. Không có `version` hoặc `path` từ stub. | Trả `CrocInfo`. Thiếu hoặc không dùng được binary báo `available: false`, kèm thông báo an toàn nếu cần. Không đồng nghĩa peer đã kết nối. |
| CC-API-02 | `StartSend(paths)` → `startSend(paths)` | `paths: string[]` là các đường dẫn nguồn cục bộ, ít nhất một giá trị không rỗng | Promise reject từ `ErrNotIntegrated`. Không có ID hoặc event. | Kiểm tra nguồn trước khi mở phiên. Promise resolve ID không rỗng nếu chấp nhận, reject nếu đầu vào sai, đang bận hoặc không khởi tạo được. |
| CC-API-03 | `StartReceive(code, outputDir)` → `startReceive(code, outputDir)` | `code: string` là mã kết nối, `outputDir: string` là thư mục đích cục bộ có quyền ghi | Promise reject từ `ErrNotIntegrated`. Lỗi stub không nhắc lại mã nhập. | Kiểm tra dữ liệu và đích trước khi mở phiên. Resolve ID nếu chấp nhận, reject nếu không khởi tạo được. Nếu mã không kết nối được sau khi có ID, báo lỗi bằng event của phiên. |
| CC-API-04 | `CancelTransfer(transferID)` → `cancelTransfer(transferID)` | `transferID: string` lấy từ kết quả bắt đầu | Go trả `false, ErrNotIntegrated`. Phía Wails nhận lỗi qua Promise reject, không phải resolve `false` thành công. | `true` nghĩa là đã chấp nhận yêu cầu hủy. `false` nếu sai ID hoặc phiên không còn hoạt động. `CANCELLED` mới là kết quả cuối của việc hủy. Lỗi vận hành reject. |
| CC-API-05 | `runtime.EventsOn("transfer:update", handler)` → `subscribe(handler)` | Handler nhận một `TransferUpdate` | Adapter đăng ký event và trả hàm hủy đăng ký. Backend stub không phát event. | Một đối tượng cập nhật cho mỗi event. UI gỡ listener khi không còn dùng. |

Đường dẫn có khoảng trắng và Unicode được chuyển thành giá trị riêng, không nối vào chuỗi lệnh. Quy tắc chuẩn hóa đường dẫn, ghi đè và xử lý mã sai chi tiết phải chốt trước tích hợp. Kết quả `Start*` chỉ là xác nhận tạo phiên, không phải xác nhận đã truyền thành công.

## 3. Từ điển dữ liệu

| Đối tượng / trường | Kiểu | Bắt buộc | Ý nghĩa |
| --- | --- | --- | --- |
| `CrocInfo.available` | boolean | Có | Croc dùng được theo điều kiện kiểm tra của dịch vụ. |
| `CrocInfo.version` | string | Không | Phiên bản đã phát hiện, không mặc định bằng pin nếu chưa kiểm tra. |
| `CrocInfo.path` | string | Không | Đường dẫn binary cục bộ nếu biết. Không đưa đường dẫn thật vào báo cáo công khai. |
| `CrocInfo.message` | string | Không | Mô tả khả dụng hoặc gợi ý khắc phục. |
| `TransferUpdate.transferID` | string | Có | ID bất biến trong một phiên. Client xử lý như chuỗi opaque, không dựa vào dạng `mock-*`. |
| `TransferUpdate.state` | enum | Có | Một giá trị trong vòng đời bên dưới. |
| `TransferUpdate.currentFile` | string | Không | Mục đang xử lý khi biết. Có thể chứa thông tin tên tệp. |
| `TransferUpdate.bytesTransferred`, `bytesTotal` | number, Go `*int64` | Không | Byte không âm khi có số đo. Không mặc định 0 cho giá trị chưa biết. |
| `TransferUpdate.bytesPerSecond` | number, Go `*int64` | Không | Byte mỗi giây khi có số đo phù hợp. |
| `TransferUpdate.progress` | number, Go `*float64` | Không | Phần trăm 0–100 khi đáng tin cậy. |
| `TransferUpdate.message` | string | Không | Thông báo đã lọc dữ liệu nhạy cảm, không kèm output thô. |
| `TransferUpdate.connectionCode` | string | Không | Chỉ chuyển mã sang UI người gửi khi cần hiển thị hoặc sao chép. |

Tên trường phân biệt hoa thường. Theo struct Go, trường chuỗi tùy chọn rỗng hoặc con trỏ nil được bỏ qua. Con trỏ có giá trị 0 được serialize thành 0. Với các trường số lớn hơn giới hạn số nguyên chính xác của JavaScript, cách biểu diễn chưa chốt. Không đổi kiểu wire khi chưa phối hợp FE và BE.

Ví dụ cập nhật mục tiêu, không chứa mã kết nối:

```json
{"transferID":"example-id","state":"WAITING_FOR_PEER"}
```

```json
{"transferID":"example-id","state":"TRANSFERRING","progress":0}
```

## 4. Vòng đời, đồng thời và lỗi

- `IDLE` là trạng thái UI chưa có phiên. Luồng thường là `PREPARING` → `WAITING_FOR_PEER` → `TRANSFERRING` → `COMPLETED`. Phiên đang chạy có thể kết thúc bằng `FAILED` hoặc `CANCELLED`. Cập nhật tiến độ có thể lặp trong cùng trạng thái.
- **Mục tiêu.** Tối đa một phiên hoạt động toàn ứng dụng. Một phiên đã chấp nhận phải có đúng một trạng thái kết thúc. Không phát cập nhật sau trạng thái đó. UI đăng ký event trước khi gọi `Start*` và ghép đúng `transferID`. Trường hợp event đầu đến trước lúc Promise resolve phải được thiết kế và kiểm thử ở CC-01/03. Bản init chưa xác nhận thứ tự qua Wails thật.
- Lỗi trước khi chấp nhận phiên đi qua Promise reject. Lỗi sau khi chấp nhận đi qua `FAILED`. Thông báo dùng cho người đọc, không parse theo câu chữ. Chưa có enum lỗi ổn định trên wire.
- Mã nhận không được xuất hiện trong event phục vụ người nhận, log hoặc thông báo lỗi. Mã người gửi chỉ dùng trong phiên. Không ghi lâu dài. Việc hủy phải dừng tiến trình con và giải phóng tài nguyên trước khi công bố `CANCELLED`.

## 5. Tương thích và kiểm thử

Tên binding, đối số theo thứ tự, tên event, trường bắt buộc và giá trị enum là ranh giới tương thích. Trường tùy chọn mới phải cập nhật cả hai kiểu dữ liệu và test deserialize. Thay tên, kiểu, giá trị hoặc thứ tự cần xác nhận trước của hai phía, cập nhật ICD và hồi quy adapter, bridge, serialization. Không thêm URL hay Swagger cho một giao diện không dùng HTTP.

Điểm kiểm thử nằm ở [ca CC-T-01/02/13/14](verification.md) và [ma trận truy vết](traceability.md). CC-01 xác nhận hợp đồng, CC-02/03 tích hợp Croc, CC-04–06 hoàn thiện UI và CC-07/08 xác minh bản Windows, theo phân công trong hồ sơ init.
