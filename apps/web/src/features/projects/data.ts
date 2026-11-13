export interface Project { id: string; name: string; region: string; client: string; budget: number; date: string; description: string; local?: boolean }
export const projectRegions = ['北京市', '上海市', '广东省', '浙江省', '江苏省']
export const projectSamples: Project[] = [
  { id: 'project-001', name: '示例园区环境检测服务', region: '北京市', client: '示例园区管理单位', budget: 38, date: '2026-11-13', description: '园区环境检测服务的演示项目，用于展示项目资料查询与详情。没有真实采购或检测活动。' },
  { id: 'project-002', name: '示例学校饮用水检测', region: '上海市', client: '示例学校后勤单位', budget: 12, date: '2026-11-12', description: '学校饮用水检测的虚构资料条目，不包含实际采购需求。' },
  { id: 'project-003', name: '示例办公楼空气检测', region: '广东省', client: '示例物业管理单位', budget: 16.5, date: '2026-11-11', description: '展示室内空气检测项目的基础字段。所有关联组织均为示例。' },
  { id: 'project-004', name: '示例实验室设备校准', region: '浙江省', client: '示例实验室', budget: 24, date: '2026-11-10', description: '用于演示实验室仪器校准服务的项目档案。' },
  { id: 'project-005', name: '示例建筑材料检测', region: '江苏省', client: '示例工程管理单位', budget: 48, date: '2026-11-09', description: '建筑材料检测项目的示例信息，不作为工程验收记录。' },
  { id: 'project-006', name: '示例食品安全检测服务', region: '北京市', client: '示例食品采购单位', budget: 32, date: '2026-11-08', description: '用于展示食品检测类项目的检索与地区分类。' },
  { id: 'project-007', name: '示例河道水质监测', region: '浙江省', client: '示例河道管理单位', budget: 56, date: '2026-11-07', description: '水环境检测的虚构项目。无真实监测数据或结论。' },
  { id: 'project-008', name: '示例纺织品检测', region: '广东省', client: '示例消费品采购单位', budget: 18, date: '2026-11-06', description: '演示消费品检测服务的项目详情结构。' },
  { id: 'project-009', name: '示例产业园噪声监测', region: '上海市', client: '示例产业园运营单位', budget: 21, date: '2026-11-05', description: '仅供模板审阅的噪声监测项目记录。' },
]
