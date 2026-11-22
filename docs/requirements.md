# SRS — Yêu cầu phần mềm

**CC-SRS-001 · 0.5 · 2026-11-19 · Dự thảo thiết kế.** Cấu trúc tham chiếu [ISO/IEC/IEEE 29148:2018](https://www.iso.org/standard/72089.html). Các yêu cầu dưới đây là mục tiêu, không phải hành vi đã triển khai. Bản init chỉ có mock và stub.

CrocCoc giúp một người gửi và một người nhận truyền tệp qua Croc trên Windows. Người gửi chia sẻ mã qua kênh do họ chọn. Người nhận chọn thư mục, xem đề nghị rồi quyết định có tải hay không. Mã không xác thực danh tính người gửi.

## Vạch cắt MVP

- **MVP nội bộ:** Windows 11 x64, hai máy thử, một phiên, gửi tệp/thư mục thông thường, NTFS cục bộ, nhận có xác nhận, không ghi đè, hủy có phản hồi, kiểm SHA-256 qua relay thực. Giao diện tiếng Việt, Windows có WebView2. Chỉ mở truyền thật sau khi qua cổng kỹ thuật trong [kiến trúc](architecture.md).
- **Chưa thuộc MVP nội bộ:** LAN, cấu hình relay, resume, tự đổi tên, hợp nhất thư mục, đa phiên, hỗ trợ đường dẫn dài, đo throughput và công bố chỉ tiêu hiệu năng. Nút sao chép, số liệu tăng dần và mở Explorer được bật khi khả năng tương ứng đã được kiểm. Luồng hiển thị và truyền mã thủ công vẫn dùng được khi chưa bật nút sao chép.
- **Phát hành công khai:** Có điều kiện riêng cho Windows 10/11 x64, WebView2, bảo mật phụ thuộc, gói cài và bằng chứng trên máy sạch. Mốc 25/12/2026 là tham chiếu lịch sử, không phải cam kết. Xem [phát hành](release.md).

## Yêu cầu chức năng

ID cũ được giữ. Các ID có hậu tố tách từng tiêu chí từ nhóm cũ, còn mã nhóm là đầu mục lịch sử. Không dùng ID ticket Notion chưa xác minh làm nguồn chuẩn. [RTM](traceability.md) nối từng yêu cầu tới kiểm thử.

| ID | Yêu cầu có thể nghiệm thu |
| --- | --- |
| CC-FR-01 | Chỉ báo Croc khả dụng khi thành phần thực thi khớp pin và định danh đã duyệt. Sai nguồn, sai pin hoặc probe hết hạn phải chặn phiên mới. |
| CC-FR-02 | Chọn và bỏ tệp/thư mục bằng giao diện. Hủy picker không đổi danh sách. |
| CC-FR-03a | Hiện mã của đúng phiên gửi đang chờ, xóa khi đã ghép peer, đang hủy, hết hạn hoặc rời màn hình. |
| CC-FR-03b | Nếu bật Sao chép, chỉ ghi khi người dùng yêu cầu và phải loại khỏi Win+V lẫn đồng bộ Windows. Không hỗ trợ thì không ghi. |
| CC-FR-04a | Đầu nhận phải từ chối đích ngoài phạm vi hỗ trợ trước khi bắt đầu phiên. |
| CC-FR-04b | Hiện danh sách tên tương đối, số tệp và tổng byte từ manifest đã kiểm. Chưa có đồng ý rõ ràng thì không yêu cầu hoặc ghi nội dung tệp. |
| CC-FR-04c | Phát hiện mục trùng trong manifest và xung đột với đích trước tải. Khi trùng, cho chọn thư mục đích khác bằng phiên mới. Không ghi đè, gộp, tự đổi tên hoặc bỏ qua. |
| CC-FR-04d | Công bố từng mục cấp cao nhất bằng thao tác no-replace sau xác minh. Ca trùng muộn phải giữ nguyên mục có sẵn. |
| CC-FR-04e | Nếu có Mở thư mục, chỉ mở đích đã xác minh của phiên nhận thành công. Lỗi Explorer không đổi kết quả truyền. |
| CC-FR-05a | Chỉ một phiên giữ tài nguyên trong một instance. Lần mở ứng dụng thứ hai trong phạm vi Windows được hỗ trợ không được mở phiên mới. |
| CC-FR-05b | UI nhận snapshot có ID và revision, khôi phục snapshot nếu bỏ lỡ event, không lùi trạng thái hoặc sinh terminal thứ hai. |
| CC-FR-05c | Không đoán phần trăm. Chỉ hiện khi có hai số byte đáng tin cậy của cùng phép đo và tổng lớn hơn 0. |
| CC-FR-05d | Hết thời gian chờ peer hoặc chờ người nhận quyết định thì đóng phiên, bỏ mã và giải phóng tài nguyên. |
| CC-FR-06 | Hủy được thể hiện ngay qua `CANCELLING`. Chỉ báo `CANCELLED` sau khi dừng và dọn xong. Hủy sau mốc khóa công bố không có hiệu lực. |
| CC-FR-07 | Báo lỗi an toàn bằng mã ổn định và thông báo tiếng Việt có hướng xử lý. Lỗi trước chấp nhận phiên không tạo ID giả. |
| CC-FR-08 | Không gửi mục hoặc cây nguồn chứa symlink, junction hay reparse point. Nếu nguồn không đọc được, bị khóa hoặc không thể giữ ổn định khi truyền, không báo thành công. |

## Phi chức năng

| ID | Yêu cầu có thể nghiệm thu |
| --- | --- |
| CC-NFR-01 | Không để mã trong argv, cấu hình, log, telemetry, crash upload hay artifact. Chỉ qua các kênh liệt kê trong [mô hình đe dọa](threat-model.md). |
| CC-NFR-02 | Cùng gói phát hành chạy trên Windows 10 và 11 x64 có WebView2 và đúng engine đã duyệt. MVP nội bộ chỉ yêu cầu Windows 11 x64. |
| CC-NFR-03 | Chỉ báo nhận hoàn tất sau xác minh nội dung theo giao thức, công bố đủ mục và dọn tài nguyên. QA so đường dẫn, số byte và SHA-256 hai đầu trên tệp đơn và nhiều mục. |
| CC-NFR-04 | Chỉ ghi nội dung nhận vào vùng staging đã cô lập trước publish. Từ chối tên không an toàn hoặc vượt giới hạn. |
| CC-NFR-05 | Luồng chọn, xác nhận, từ chối và hủy dùng được bằng bàn phím. Trạng thái và lỗi quan trọng được trình đọc màn hình thông báo, không đọc mã tự động. |
| CC-NFR-06 | Nhật ký chẩn đoán chỉ chứa dữ liệu đã lọc như phiên bản engine, mã lỗi, thời gian và pha. Người dùng xem trước nội dung xuất. |
