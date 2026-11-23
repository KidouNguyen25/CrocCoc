# ICD — Hợp đồng truyền tệp

**CC-ICD-001 · 0.6 · 2026-11-23 · Đang triển khai trên Windows.**

Đây là Wails IPC giữa [bridge Go](../app.go) và [adapter TS](../frontend/src/adapters/wailsTransfer.ts), không phải API HTTP. Host hiện triển khai 8 binding: `GetCrocInfo` (CC-API-01), `StartSend` (CC-API-02), `StartReceive` (CC-API-03), `CancelTransfer` (CC-API-04), `GetTransferSnapshot` (CC-API-06, trả `*Update` / `null`), `SelectPaths` (CC-API-07), `CopySendCode` (CC-API-08), `DecideReceive` (CC-API-10), cùng sự kiện `transfer:update` (CC-API-05). Go wire (`backend/transfer/contract.go`) đã bỏ `Idle`, bỏ `progress`/`bytesPerSecond`, bỏ `CrocInfo.Path`, và gửi cặp `bytesTransferred`/`bytesTotal` dạng chuỗi thập phân; UI giữ `IDLE` cục bộ và tự tính phần trăm bằng số nguyên chính xác (`BigInt`). Wire hiện tại cho lỗi trước chấp nhận dùng chuỗi Go chuẩn hóa `"CODE: message"` (được khóa bằng unit test). Hai mục `OpenReceiveDirectory` (CC-API-09) và Wails `ErrorFormatter` trả object `{code, message}` cho reject được ghi nhận là **nợ kỹ thuật sau MVP**.

## Lời gọi và sự kiện mục tiêu

| ID | Binding / sự kiện | Hành vi |
| --- | --- | --- |
| CC-API-01 | `GetCrocInfo()` | Trả khả dụng và phiên bản đã xác minh. Trả `{available, version?, error?}`. Nếu unavailable phải có `error` theo dạng bên dưới, không lộ đường dẫn máy. |
| CC-API-02 | `StartSend(paths)` | Chấp nhận danh sách hợp lệ, cấp ID nếu mở được phiên. Không ghép đường dẫn vào chuỗi shell. |
| CC-API-03 | `StartReceive(code, outputDir)` | Kiểm mã/đích cục bộ rồi cấp ID. Chỉ bắt đầu kết nối để nhận metadata, chưa cho tải nội dung. Sau khi nhận lời gọi, UI bỏ mã khỏi ô nhập. |
| CC-API-04 | `CancelTransfer(transferID)` | `true` khi thắng mốc khóa và phiên đi vào `CANCELLING`. `false` nếu sai ID, đã yêu cầu hủy, đang công bố hoặc đã kết thúc. |
| CC-API-05 | `transfer:update` | Một event chứa đúng một snapshot đầy đủ. Adapter trả hàm tháo listener. |
| CC-API-06 | `GetTransferSnapshot(transferID)` | Snapshot của phiên hoạt động hoặc terminal gần nhất trong RAM, hoặc `null`. Không lưu lịch sử. Khi chấp nhận phiên mới, không bảo đảm còn terminal cũ. |
| CC-API-07 | `SelectPaths(kind)` | `sendFiles`, `sendDirectory`, `receiveDirectory`. Hủy picker trả mảng rỗng, loại thư mục trả tối đa một đường dẫn. |
| CC-API-08 | `CopySendCode(transferID)` | Tùy khả năng. Chỉ host chép đúng mã đang hiển thị cho phiên gửi còn chờ ở thời điểm ghi. Không nhận chuỗi tùy ý hay ghi lặp ngoài thao tác người dùng. Đây là ràng buộc tính toàn vẹn, không phải bảo mật mã khỏi WebView. Nếu cờ Win32 chưa xác minh được, trả lỗi `COPY_UNSAFE` và không ghi mã. |
| CC-API-09 | `OpenReceiveDirectory(transferID)` | Tùy khả năng. Chỉ mở đích vừa xác minh lại của phiên nhận thành công gần nhất. ID sai trả `false`, lỗi Explorer trả lỗi riêng. |
| CC-API-10 | `DecideReceive(transferID, offerID, accept)` | So khớp đề nghị bất biến còn hạn. `true` nếu quyết định được nhận, `false` nếu sai ID, đã quyết định, hết hạn hoặc không còn ở pha đợi. Không nhận tên hoặc đường dẫn từ UI. Khi `accept=true`, backend chạy lại preflight trước khi cho tải. Từ chối đi qua `CANCELLING` rồi `CANCELLED` nếu dọn xong. |

Kết quả `Start*` xác nhận chấp nhận phiên, không xác nhận truyền thành công. Trên wire MVP hiện hành, lỗi trước chấp nhận (`Start*`, `SelectPaths`, `CopySendCode`, `DecideReceive`) trả reject dạng chuỗi chuẩn hóa `"CODE: message"` với tiền tố `ErrorCode` cố định được khóa bằng kiểm thử tự động, không phát ID hoặc event; việc chuyển reject sang object `{code, message}` qua [ErrorFormatter của Wails](https://v2.wails.io/docs/reference/options/) cùng binding `CC-API-09 OpenReceiveDirectory` là nợ sau MVP. Các lỗi sau chấp nhận đi qua trạng thái `FAILED` và trường `error: {code, message}` có cấu trúc trên cả event `transfer:update` lẫn `GetTransferSnapshot`. UI phân nhánh theo mã `ErrorCode`, không dựa vào dấu câu hoặc câu tiếng Việt.

| Nhóm | `ErrorCode` tối thiểu |
| --- | --- |
| Điều kiện bắt đầu | `NOT_INTEGRATED`, `BUSY`, `INVALID_INPUT`, `ENGINE_UNAVAILABLE`, `UNSUPPORTED_DESTINATION`, `SOURCE_UNAVAILABLE` |
| Nhận và thời hạn | `UNSAFE_OFFER`, `DESTINATION_CONFLICT`, `RESOURCE_LIMIT`, `PEER_TIMEOUT`, `OFFER_TIMEOUT` |
| Vận hành | `SOURCE_CHANGED`, `TRANSFER_FAILED`, `PUBLISH_FAILED`, `CLEANUP_FAILED`, `COPY_UNSAFE`, `OS_ACTION_FAILED` |

## Snapshot và vòng đời

Bắt buộc có `transferID` opaque, `revision` số nguyên an toàn JS tăng từ 1 trong mỗi phiên, và `state`. Trường tùy chọn không gửi `null`. Thiếu trường nghĩa là không còn giá trị, không dùng lại từ snapshot trước.

- `receiveOffer` chỉ có khi `AWAITING_ACCEPTANCE`. Gồm `offerID`, danh sách tên tương đối đã kiểm kèm loại mục và số byte của tệp, `fileCount`, `totalBytes` và thời điểm hết hạn. Không chứa đường dẫn máy gửi. Revision không thay cho `offerID`.
- `connectionCode` chỉ có ở snapshot gửi còn `WAITING_FOR_PEER`, qua CC-API-05/06 có chủ đích. Không trả trong trạng thái khác, reject, snapshot chẩn đoán hay API clipboard. Đọc [ngoại lệ bảo mật](threat-model.md).
- `bytesTransferred`, `bytesTotal` là cặp tùy chọn cùng phạm vi đo của phiên, dạng chuỗi thập phân chuẩn không âm, tối đa `9223372036854775807`, với số đã truyền không vượt tổng. Có thể bỏ cả cặp. Không gửi `progress` hoặc `bytesPerSecond` trên wire mục tiêu. UI tính phần trăm từ cặp bằng số nguyên chính xác trước khi đổi sang số hiển thị. Tổng bằng 0 hoặc thiếu số đo thì tiến độ không xác định. `100%` không có nghĩa đã publish xong.
- `error` có `code` và `message` đã lọc, chỉ có ở `FAILED`. `partialOutput: true` nếu công bố đã thay đổi đích và rollback không xác minh là sạch. `cleanupPending: true` nếu còn tài nguyên chưa giải phóng. Cả hai không chứa đường dẫn hoặc output thô.

| Trạng thái | Chuyển tiếp hợp lệ |
| --- | --- |
| `PREPARING` | `WAITING_FOR_PEER`, `CANCELLING`, `FAILED` |
| `WAITING_FOR_PEER` | `AWAITING_ACCEPTANCE` (nhận), `TRANSFERRING` (gửi sau khi peer đồng ý), `CANCELLING`, `FAILED` |
| `AWAITING_ACCEPTANCE` | `TRANSFERRING`, `CANCELLING`, `FAILED` |
| `TRANSFERRING` | `FINALIZING`, `CANCELLING`, `FAILED` |
| `FINALIZING` | `COMPLETED`, `FAILED` |
| `CANCELLING` | `CANCELLED`, `FAILED` |
| `COMPLETED`, `FAILED`, `CANCELLED` | Terminal, không phát thêm snapshot cho ID này. |

Backend ghi `CANCELLING` và bỏ mã trước khi đợi engine dừng. `FINALIZING` là mốc khóa, đứng trước publish đầu tiên hoặc chốt thành công gửi. Timeout dùng cùng quy trình dừng nhưng kết thúc `FAILED` với lý do timeout nếu dọn thành công. Dọn không xác nhận được trong hạn thì `FAILED / CLEANUP_FAILED`, chặn phiên mới cho tới khi tài nguyên đã nhả. Không có terminal thứ hai. Khi đóng cửa sổ (`OnBeforeClose`), host hỏi xác nhận hủy phiên nếu đang `TRANSFERRING`, hoặc cảnh báo rõ phiên đã qua mốc khóa `FINALIZING` (đang công bố tệp, không thể hủy ở bước này và thoát lúc này có thể để lại kết quả một phần; việc `Shutdown` chỉ chờ tối đa `WorkerCleanupTimeout` thay vì đợi hết toàn bộ publish cùng luồng hiển thị trạng thái "đang dọn" bất đồng bộ trên UI được ghi nhận là nợ sau MVP). Trong preflight `StartReceive`, bước kiểm tra `CheckReceiveDestinationEligibility` chạy trên goroutine tách biệt để `Shutdown` hoặc `PreparationTimeout` có thể hủy chờ ngay và giữ ghim worker khi quá hạn; nếu lời gọi hệ thống tệp của OS bị treo vô hạn thì goroutine chỉ-đọc này có thể còn sống đến khi tiến trình thoát (ghi nhận là nợ nhỏ sau MVP).

UI đăng ký listener trước Start, nhận ID rồi đọc snapshot. Chỉ áp dụng ID hiện tại và revision mới hơn, bỏ ID cũ và không lùi khỏi terminal. Khi mở lại listener hoặc thiếu cập nhật, đọc lại snapshot với retry hữu hạn, báo mất theo dõi nếu không phục hồi. Adapter bỏ payload sai ID, revision hoặc state. Cặp byte sai bị bỏ, không làm mất terminal hợp lệ. Xem [các ca native](verification.md).
