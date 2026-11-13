export interface Standard { id: string; code: string; name: string; category: string; status: string; published: string; implemented: string; summary: string }
export const standardCategories = ['检测与校准', '质量管理', '环境检测', '食品检测']
// These identifiers and descriptions are deliberately illustrative, not an authoritative standards catalog.
export const standards: Standard[] = [
  { id: 'standard-001', code: 'GB/T 示例 001-2026', name: '检测与校准实验室能力要求', category: '检测与校准', status: '现行（示例）', published: '2026-01-10', implemented: '2026-07-01', summary: '展示实验室能力要求类资料的查询与归档结构。内容为演示摘要，不是标准原文或合规依据。' },
  { id: 'standard-002', code: 'GB/T 示例 002-2026', name: '质量管理体系基础与术语', category: '质量管理', status: '现行（示例）', published: '2026-02-01', implemented: '2026-08-01', summary: '用于展示质量管理分类下的标准条目。正式术语及版本请查询权威发布平台。' },
  { id: 'standard-003', code: 'GB/T 示例 003-2025', name: '地表水采样质量控制指南', category: '环境检测', status: '现行（示例）', published: '2025-09-01', implemented: '2026-03-01', summary: '用于演示环境水质采样资料的字段和分类，不提供实际采样方法或检测结论。' },
  { id: 'standard-004', code: 'GB 示例 004-2026', name: '食品微生物检测资料规范', category: '食品检测', status: '即将实施（示例）', published: '2026-10-01', implemented: '2027-04-01', summary: '食品检测分类的示例条目。这里的日期与状态均为虚构演示数据。' },
  { id: 'standard-005', code: 'GB/T 示例 005-2024', name: '实验室测量不确定度评定指南', category: '检测与校准', status: '已替代（示例）', published: '2024-03-01', implemented: '2024-09-01', summary: '演示历史版本状态。正式测量不确定度评定请参考有效标准及专业技术文件。' },
  { id: 'standard-006', code: 'GB/T 示例 006-2026', name: '环境空气检测记录规范', category: '环境检测', status: '现行（示例）', published: '2026-03-01', implemented: '2026-09-01', summary: '展示环境空气检测记录类标准的检索与详情布局。无真实标准全文附件。' },
  { id: 'standard-007', code: 'GB/T 示例 007-2026', name: '质量管理过程审核指南', category: '质量管理', status: '现行（示例）', published: '2026-04-01', implemented: '2026-10-01', summary: '演示质量管理分类的另一条记录，用于检验组合筛选结果的一致性。' },
  { id: 'standard-008', code: 'GB 示例 008-2025', name: '食品检验样品管理规范', category: '食品检测', status: '现行（示例）', published: '2025-12-01', implemented: '2026-06-01', summary: '展示食品检验样品管理资料的示例信息，不作为真实样品管理作业文件。' },
  { id: 'standard-009', code: 'GB/T 示例 009-2026', name: '校准结果报告编制指南', category: '检测与校准', status: '现行（示例）', published: '2026-05-01', implemented: '2026-11-01', summary: '演示报告编制类标准的编号、分类与状态字段，供模板审阅。' },
]
