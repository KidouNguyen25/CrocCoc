# SRS — Đặc tả yêu cầu phần mềm CrocCoc

| Mã | Phiên bản | Ngày | Trạng thái | Mã nguồn đối chiếu |
| --- | --- | --- | --- | --- |
| CC-SRS-001 | 0.2 | 2026-11-19 | Dự thảo để duyệt | `610b622` |

Tham chiếu [ISO/IEC/IEEE 29148:2018](https://www.iso.org/standard/72089.html) cho cấu trúc đặc tả, yêu cầu có mã, tiêu chí xác minh và truy vết. Các mã CC-01–CC-08 và mốc kế hoạch 25/12/2026 được kế thừa từ hồ sơ init. [Kế hoạch Notion](https://kidounguyen.notion.site/CrocCoc-41cf424aba708182998fe301aaed3636) không được đối chiếu trực tiếp trong lần cập nhật này.

## 1. Mục đích và phạm vi

CrocCoc giúp người dùng Windows 10/11 x64 gửi hoặc nhận tệp bằng Croc mà không cần sử dụng terminal. Người gửi chọn dữ liệu, tạo phiên và chia sẻ mã qua kênh họ chọn. Người nhận nhập mã và lưu dữ liệu vào thư mục đích.

| Vai trò hoặc hệ thống ngoài | Trách nhiệm |
| --- | --- |
| Người gửi | Chọn tệp hoặc thư mục, xác định đúng người nhận, chia sẻ mã, theo dõi hoặc hủy. |
| Người nhận | Xác minh nguồn gửi, nhập mã, chọn thư mục có quyền ghi, theo dõi hoặc hủy. |
| Croc CLI và hạ tầng mạng | Croc thực hiện giao thức truyền và dùng relay khi cần. CrocCoc đóng vai trò điều phối tại máy người dùng. |
| Windows và WebView2 | Cung cấp môi trường chạy desktop, tài nguyên hệ thống và giao diện WebView. |

Ngoài phạm vi bản đầu gồm tài khoản, lịch sử truyền lưu lâu dài, relay tự quản, giao thức riêng, Croc SSH và stored transfer. Mockup thiết kế trong `.tmp/` là nguyên mẫu duyệt giao diện, không phải bản chức năng giao cho người dùng.

## 2. Hành vi và yêu cầu

Từ **phải** áp dụng cho bản phát hành mục tiêu. `Mock` nghĩa là bản init thể hiện một phần hành vi bằng dữ liệu mẫu. `Stub` nghĩa là có ranh giới gọi nhưng chưa triển khai nghiệp vụ. Không trạng thái nào trong bảng đồng nghĩa đã nghiệm thu.

| Mã | Yêu cầu bản đầu | Tiêu chí chấp nhận | Bản init | Nguồn |
| --- | --- | --- | --- | --- |
| CC-FR-01 | Ứng dụng phải xác định khả dụng và phiên bản của Croc. | Chỉ binary đã khóa được báo khả dụng. Thiếu, không chạy được hoặc sai phiên bản phải có thông báo và hướng xử lý. | Stub | CC-02 |
| CC-FR-02 | Người gửi phải chọn và bỏ được tệp hoặc thư mục trước khi gửi. | Chọn nhiều mục, hiển thị tên và dung lượng theo thiết kế, rồi bỏ một mục. Dữ liệu gửi không chứa mục đã bỏ. | Mục mẫu cố định | CC-03, CC-05 |
| CC-FR-03 | Phiên gửi phải hiển thị mã do Croc phát và cho phép sao chép mã. | Nội dung clipboard khớp mã hiển thị. Máy nhận dùng chính mã này để tham gia phiên. | Mock hiển thị mã mẫu | CC-03, CC-05 |
| CC-FR-04 | Người nhận phải nhập mã, chọn nơi lưu và mở thư mục đích sau khi hoàn tất. | Dữ liệu được ghi vào thư mục đã chọn và thao tác mở đến đúng nơi. Mã hoặc thư mục không hợp lệ được báo lỗi. | Mã và thư mục mẫu cố định | CC-03, CC-06 |
| CC-FR-05 | Ứng dụng phải theo dõi một phiên hoạt động tại một thời điểm bằng ID phiên. | Lời bắt đầu thứ hai bị từ chối. Mọi cập nhật có đúng ID. Mỗi phiên chỉ có một trạng thái cuối. Chỉ hiện phần trăm nếu đo được. | Mock | CC-01, CC-03–CC-06 |
| CC-FR-06 | Người dùng phải hủy được phiên gửi và nhận khi chờ hoặc đang truyền. | Phiên đến `CANCELLED`, tiến trình con đã kết thúc, người dùng có thể tạo phiên mới và không có cập nhật muộn của phiên đã hủy. | Mock hủy lịch sự kiện | CC-03, CC-05, CC-06 |
| CC-FR-07 | Ứng dụng phải báo lỗi dễ hiểu khi phiên đã mở thất bại. | Khi runner mất kết nối hoặc thoát lỗi trong môi trường kiểm thử, phiên báo `FAILED`, UI vẫn phản hồi và nêu bước xử lý. Lỗi trước khi mở phiên không tạo ID giả. | Stub trả lỗi chưa tích hợp | CC-03, CC-08 |
| CC-NFR-01 | Ứng dụng không được ghi mã kết nối vào log, cấu hình, lịch sử, telemetry hoặc đối số hiển thị trên dòng lệnh. | Rà đầu ra và thông tin tiến trình ở các ca thành công, thất bại và hủy bằng mã kiểm thử. Runner không dùng shell và dùng cơ chế truyền bí mật được phiên bản Croc đã khóa hỗ trợ. | Chưa có runner, lỗi stub không phản chiếu mã | CC-02, CC-03, CC-08 |
| CC-NFR-02 | Bản phân phối phải chạy trên Windows 10 và 11 x64 với WebView2 và đúng phiên bản Croc đã khóa. | Mở bản đóng gói trên cả hai hệ điều hành, UI tải được và binding trả phiên bản Croc khớp hồ sơ bản build. | Cấu hình Wails, chưa nghiệm thu native | CC-07, CC-08 |
| CC-NFR-03 | Khi phiên báo hoàn tất, dữ liệu đích phải giữ đúng nội dung và cấu trúc dự kiến. | Đối chiếu danh sách, đường dẫn tương đối, dung lượng và SHA-256 với tệp đơn, tập nhiều mục và tệp lớn. Có ít nhất một ca qua relay trong môi trường đã chuẩn bị. | Chưa truyền thật | CC-08 |

## 3. Luồng sử dụng và điều kiện biên

- **Gửi.** Người dùng chọn dữ liệu và xác nhận. Nếu nguồn hợp lệ và Croc khả dụng, ứng dụng bắt đầu phiên và hiển thị mã khi Croc phát. Kết thúc chỉ khi phiên hoàn tất, thất bại hoặc bị hủy.
- **Nhận.** Người dùng nhập mã và chọn đích. Nếu thông tin hợp lệ, ứng dụng mở phiên nhận, theo dõi trạng thái và cho mở thư mục đích sau khi hoàn tất.
- **Hủy hoặc thất bại.** Người dùng được biết phiên nào kết thúc và lý do có thể công bố. Không mô tả trạng thái lỗi là thành công hoặc tự động bắt đầu lại phiên.
- **Mất điều kiện.** Binary thiếu, nguồn biến mất trước khi gửi, đích mất quyền ghi hoặc phiên đang hoạt động phải được xử lý rõ. Cách xử lý tệp dở khi gián đoạn cần chốt trước QA truyền thật.

## 4. Ràng buộc và quyết định còn mở

| Mục | Chủ thể chốt theo phạm vi hiện hữu | Điều kiện cần chốt |
| --- | --- | --- |
| Chính sách ghi đè, dữ liệu dở, phục hồi sau hủy | Backend và QA | Trước ca nhận đích có dữ liệu hoặc ca mất mạng. |
| Kích thước fixture “tệp lớn”, mạng và điều kiện timeout | QA và backend | Trước khi chạy ca relay. Ghi giá trị thực vào hồ sơ chạy, không coi fixture là giới hạn sản phẩm. |
| Cách nhận diện và đóng gói binary được hỗ trợ | Backend và tích hợp | Trước khi nghiệm thu CC-FR-01 và CC-NFR-02. Bản init ghi nhận pin Croc `11.6.4`, chưa chứng minh binary có mặt. |

Ánh xạ từng yêu cầu đến điểm giao diện, thành phần và ca xác minh nằm trong [ma trận truy vết](traceability.md). Đổi phạm vi hoặc tiêu chí thì cập nhật SRS, tài liệu liên quan và ca kiểm thử cùng thay đổi.
