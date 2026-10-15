import type { components } from '../api/generated/schema';
type Address = components['schemas']['Address'];
type Region = components['schemas']['Region'];

/**
 * addressRegionLabel 用途：生成完整省市区名称，供提交前核对。
 * 参数：address 为当前地址；levels 为省、市、区三级候选项。
 * 返回值：完整地区名称；选项未加载或父子关系不匹配时返回空字符串。
 */
export function addressRegionLabel(address: Address, levels: Region[][]): string {
  const codes = [address.province_code, address.city_code, address.district_code];
  const regions = codes.map((code, i) => levels[i]?.find((region) => region.code === code));
  if (regions.some((region) => !region)) return '';
  if (regions[1]!.parent_code !== codes[0] || regions[2]!.parent_code !== codes[1]) return '';
  return regions.map((region) => region!.name).join(' ');
}
