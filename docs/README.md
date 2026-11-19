# Bộ hồ sơ kỹ thuật CrocCoc

| Mã bộ hồ sơ | Phiên bản | Ngày | Mã nguồn đối chiếu | Trạng thái |
| --- | --- | --- | --- | --- |
| CC-DOC-001 | 0.2 | 2026-11-19 | `610b622` | Dự thảo để đội dự án duyệt |

CrocCoc là ứng dụng desktop. Frontend và backend tách thành phần và kiểm thử, tích hợp qua Wails IPC. Bản init hiện có mock gửi/nhận, adapter và backend stub, chưa truyền thật.

## Danh mục

| Mã | Tài liệu | Nội dung | Tham chiếu chính |
| --- | --- | --- | --- |
| CC-SRS-001 | [Đặc tả yêu cầu phần mềm](requirements.md) | Vai trò, phạm vi, yêu cầu, tiêu chí chấp nhận và quyết định mở | ISO/IEC/IEEE 29148:2018 |
| CC-ICD-001 | [Đặc tả API và điều khiển giao diện](transfer-contract.md) | Lời gọi Wails, kiểu dữ liệu, lỗi, sự kiện, vòng đời và tương thích | ISO/IEC/IEEE 29148:2018 và 15289:2019 |
| CC-AD-001 | [Mô tả kiến trúc](architecture.md) | Bên liên quan, góc nhìn, thành phần, giao tiếp, triển khai và ADR | ISO/IEC/IEEE 42010:2022 |
| CC-VV-001 | [Kế hoạch và đặc tả kiểm thử](verification.md) | Môi trường, dữ liệu, thủ tục, tiêu chí đạt, trạng thái và hồ sơ kiểm thử init | ISO/IEC/IEEE 29119-3:2021 |
| CC-RTM-001 | [Ma trận truy vết](traceability.md) | Yêu cầu → thành phần → API → ca nghiệm thu | ISO/IEC/IEEE 29148:2018 và 29119-3:2021 |
| CC-CM-001 | [Quản lý cấu hình và thay đổi](configuration-management.md) | Hạng mục, baseline, kiểm soát thay đổi và hồ sơ trạng thái | ISO/IEC/IEEE 15289:2019 |
| CC-REL-001 | [Thủ tục build và phát hành Windows](release.md) | Điều kiện, build, kiểm chứng, bàn giao và quay lui | ISO/IEC/IEEE 15289:2019 |

Bắt đầu từ SRS để nắm phạm vi. Khi sửa frontend hoặc backend, đọc ICD và kiến trúc. Dùng RTM để chọn ca trong V&V. Dùng thủ tục phát hành khi chuẩn bị bàn giao, không dùng ZIP mockup như bản cài.

## Phạm vi áp dụng tiêu chuẩn

- [ISO/IEC/IEEE 29148:2018](https://www.iso.org/standard/72089.html) định hướng hồ sơ yêu cầu, điều kiện xác minh và truy vết.
- [ISO/IEC/IEEE 42010:2022](https://www.iso.org/standard/74393.html) định hướng mô tả kiến trúc theo bên liên quan, mối quan tâm và các góc nhìn.
- [ISO/IEC/IEEE 29119-3:2021](https://www.iso.org/standard/79429.html) định hướng cấu trúc kế hoạch, ca kiểm thử và báo cáo kết quả.
- [ISO/IEC/IEEE 15289:2019](https://www.iso.org/standard/74909.html) định hướng lựa chọn, phân loại và duy trì các hồ sơ vòng đời.

Bộ hồ sơ vận dụng các chuẩn theo phạm vi dự án. Đây không phải đánh giá đối chiếu từng điều khoản hay tuyên bố chứng nhận. `0.2` là phiên bản tài liệu, độc lập với phiên bản ứng dụng. Mã nguồn đối chiếu là baseline kỹ thuật, không chứng minh mọi mục tiêu đã triển khai. Để quản lý revision hoặc xác nhận baseline mới, dùng [quy trình cấu hình](configuration-management.md).
