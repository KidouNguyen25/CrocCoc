# SRS — Đặc tả yêu cầu CrocCoc

| ID | Phiên bản | Trạng thái | Mốc chung |
| --- | --- | --- | --- |
| CC-SRS-001 | 0.1 | Bản nháp để nhóm duyệt | 25/12/2026 |

Tài liệu vận dụng [ISO/IEC/IEEE 29148:2018](https://www.iso.org/standard/72089.html) để đặt mã yêu cầu, nêu tiêu chí chấp nhận và truy vết. Nguồn phạm vi là [kế hoạch CrocCoc](https://kidounguyen.notion.site/CrocCoc-41cf424aba708182998fe301aaed3636). Tất cả yêu cầu bản đầu dưới đây chưa được nghiệm thu trừ khi có bằng chứng riêng trong [hồ sơ kiểm thử](verification.md).

## Phạm vi

Người gửi và người nhận trao đổi tệp giữa hai máy bằng giao diện desktop Windows 10/11 x64, không thao tác terminal. Croc CLI đảm nhiệm giao thức, mã hóa và relay. Không làm tài khoản, lịch sử truyền, relay tự quản, giao thức riêng, Croc SSH hay stored transfer trong bản đầu.

## Yêu cầu bản đầu

| Mã | Yêu cầu | Tiêu chí chấp nhận | Nguồn |
| --- | --- | --- | --- |
| CC-FR-01 | Ứng dụng phải xác định khả dụng và phiên bản Croc. | Binary hợp lệ trả khả dụng và phiên bản đã khóa. Thiếu, không chạy được hoặc sai phiên bản trả lỗi có hướng xử lý. | CC-02 |
| CC-FR-02 | Người gửi phải chọn và bỏ được tệp/thư mục trước khi gửi. | Chọn nhiều mục gồm tệp và thư mục. Danh sách hiển thị tên và dung lượng theo thiết kế. Bỏ một mục và xác nhận mục đó không đi vào lệnh gửi. | CC-03, CC-05 |
| CC-FR-03 | Phiên gửi phải hiển thị mã do Croc trả về và cho phép sao chép mã. | Từ UI gửi bắt đầu phiên thật, mã sao chép khớp mã hiển thị và dùng được ở máy nhận. | CC-03, CC-05 |
| CC-FR-04 | Người nhận phải nhập mã, chọn và mở thư mục đích. | Một phiên hợp lệ ghi tệp vào đúng thư mục. Sau hoàn tất, nút mở đưa người dùng đến thư mục đó. Mã hoặc thư mục không hợp lệ báo lỗi. | CC-03, CC-06 |
| CC-FR-05 | Ứng dụng phải phản ánh vòng đời phiên qua một ID và chỉ cho một phiên hoạt động. | Lệnh thứ hai bị từ chối khi đang có phiên. Event có ID đúng và trạng thái kết thúc không bị ghi đè. Chỉ hiện phần trăm nếu có số đo tin cậy. | CC-01, CC-03 đến CC-06 |
| CC-FR-06 | Người dùng phải hủy được cả gửi và nhận. | Hủy một phiên đang chờ hoặc đang truyền đưa phiên đó về `CANCELLED`, giải phóng tiến trình và không phát cập nhật muộn. | CC-03, CC-05, CC-06 |
| CC-FR-07 | Ứng dụng phải báo lỗi có thể hiểu khi tiến trình thất bại. | Mô phỏng mất kết nối và Croc thoát lỗi. UI không treo, hiển thị `FAILED` và gợi ý cách xử lý. | CC-03, CC-08 |
| CC-NFR-01 | Không được đưa mã nhận vào log, cấu hình, lịch sử hoặc chuỗi lệnh shell. | Kiểm tra log, cấu hình, dòng lệnh và stderr công khai ở ca thành công, lỗi, hủy. Runner không ghép chuỗi shell và dùng cơ chế truyền secret được Croc hỗ trợ. | CC-02, CC-03, CC-08 |
| CC-NFR-02 | Bản phân phối phải chạy được trên Windows 10/11 x64 với phiên bản Croc đã khóa. | Build, mở ứng dụng với WebView2 trên Windows 10 và 11 x64, nhận đúng phiên bản Croc đi kèm. | CC-07, CC-08 |
| CC-NFR-03 | Truyền thành công phải giữ toàn vẹn nội dung và cấu trúc tệp. | Đối chiếu số lượng, đường dẫn tương đối, dung lượng và SHA-256 nguồn/đích cho tệp đơn, tập hợp tệp/thư mục và tệp lớn. Thử truyền khác mạng qua relay khi cần. | CC-08 |

Khi sửa tiêu chí, cập nhật đồng thời [ICD](transfer-contract.md) nếu có ảnh hưởng API và ca kiểm thử tương ứng trong [ma trận truy vết](verification.md). Không đánh dấu một yêu cầu bản đầu là đạt dựa trên mô phỏng.
