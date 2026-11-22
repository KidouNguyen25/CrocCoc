# Runbook nghiệm thu E2E và demo CrocCoc (CC-08)

Tài liệu thao tác cho [V&V](verification.md) CC-T-11 đến CC-T-16. Ghi kết quả vào mục 4 của V&V theo mẫu hồ sơ chạy; không đánh dấu `PASS` trước khi chạy thật.

## 1. Chuẩn bị

1. Build gói: `scripts\build-release.ps1` (máy Windows x64). Ghi SHA-256 của `CrocCoc-<ver>-win64.zip` và `git SHA` trong `release-manifest.json`.
2. Hai máy Windows 10/11 x64 có WebView2. Giải nén gói vào thư mục mới, giữ `CrocCoc.exe` cạnh `bin\croccoc-engine.exe`. Không cần đặt `CROCCOC_WORKER_PATH`.
3. Trên máy gửi tạo fixture (Node 24+ ở máy phát triển, không cần cài trên máy demo nếu copy sẵn fixture):

```powershell
node scripts\qa\fixtures.mjs make C:\qa\src --large-mb 512
```

Fixture gồm tệp nhỏ, cây thư mục tên Unicode có khoảng trắng và thư mục rỗng, và `large.bin`. Manifest sinh ra tại `C:\qa\src.manifest.json`. Ghi số byte từng tệp vào hồ sơ trước khi chạy.

## 2. Đối chiếu sau mỗi lần nhận

```powershell
node scripts\qa\fixtures.mjs compare C:\qa\src D:\qa\received
```

Kết luận `PASS` khi mọi tệp nguồn có mặt với cùng kích thước và SHA-256, mọi thư mục rỗng được giữ. Lệnh thoát mã 1 khi lệch. Chạy với đích sạch cho ca mặc định. Không đưa mã kết nối vào ảnh hay log lưu làm bằng chứng.

## 3. Thao tác theo ca

| Ca | Thao tác | Ghi lại |
| --- | --- | --- |
| CC-T-11 | Mở app, đọc trạng thái Croc. Thử đổi tên `bin\croccoc-engine.exe`, rồi thay bằng bản bị sửa một byte. Khôi phục bản gốc. | Cả ba trạng thái: đúng bản thì khả dụng; thiếu hoặc sai hash thì không khả dụng và có hướng xử lý. |
| CC-T-12 | Gửi `small.txt`, rồi gửi cả `Thư mục thử nghiệm` kèm `small.txt`, bỏ `small.txt` trước khi gửi, nhận bằng mã. Mở thư mục kết quả từ UI. | Ảnh đã che mã và kết quả `compare` cho từng lượt. |
| CC-T-13 | Hủy khi chờ người nhận và khi đang truyền ở cả hai vai. Mở phiên thứ hai khi phiên đầu còn chạy. Kiểm tra `Get-Process` không còn tiến trình con. | Dòng thời gian trạng thái, kết quả `Get-Process`. |
| CC-T-14 | Nhập mã sai, chọn đích không ghi được, xóa nguồn giữa chừng, rút mạng giữa lúc truyền. Rà `%LOCALAPPDATA%`, thông báo UI và log để tìm mã kết nối. | Bước tái hiện, kết luận rà bí mật. |
| CC-T-15 | Truyền `large.bin` giữa hai mạng khác nhau qua relay đã xác nhận (hai máy không cùng LAN). | `compare` PASS, thông tin xác nhận đường relay không chứa mã. |
| CC-T-16 | Chạy đúng gói đã tính SHA-256 trên Windows 10 và Windows 11. | SHA-256 artifact, bản Windows, bản WebView2, kết quả. |

Ca còn nợ theo README của worker: khôi phục sau khi khởi động lại Windows giữa lúc nhận (journal staging), và xác nhận gói cuối dùng đúng worker đã ghim. Hai mục này cần ca riêng trong hồ sơ chạy.

## 4. Kịch bản demo (khoảng 5 phút)

1. Máy A: mở CrocCoc, chọn thư mục `Thư mục thử nghiệm` và `small.txt`, bấm Gửi, sao chép mã.
2. Máy B: mở CrocCoc, nhập mã, chọn thư mục lưu, xem đề nghị nhận (danh sách tệp), chấp nhận.
3. Chạy `compare` trên máy nhận để chứng minh toàn vẹn.
4. Lượt hai: bắt đầu gửi `large.bin` rồi hủy giữa chừng, chỉ ra không còn tiến trình con.
5. Nhập mã sai để cho thấy thông báo lỗi dễ hiểu.

Giữ sẵn ảnh chụp và `release-manifest.json` của gói đang demo để đối chiếu nếu bị hỏi về phiên bản.
