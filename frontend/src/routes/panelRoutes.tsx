import { lazy } from 'react';
import type { ComponentType } from 'react';
import {
  ApiOutlined,
  ClusterOutlined,
  CrownOutlined,
  DashboardOutlined,
  ExportOutlined,
  GlobalOutlined,
  ImportOutlined,
  SafetyOutlined,
  SettingOutlined,
  SwapOutlined,
  TagsOutlined,
  TeamOutlined,
  ToolOutlined,
  UserSwitchOutlined,
} from '@ant-design/icons';

const IndexPage = lazy(() => import('@/pages/index/IndexPage'));
const InboundsPage = lazy(() => import('@/pages/inbounds/InboundsPage'));
const ClientsPage = lazy(() => import('@/pages/clients/ClientsPage'));
const GroupsPage = lazy(() => import('@/pages/groups/GroupsPage'));
const NodesPage = lazy(() => import('@/pages/nodes/NodesPage'));
const HostsPage = lazy(() => import('@/pages/hosts/HostsPage'));
const SettingsPage = lazy(() => import('@/pages/settings/SettingsPage'));
const XrayPage = lazy(() => import('@/pages/xray/XrayPage'));
const ApiDocsPage = lazy(() => import('@/pages/api-docs/ApiDocsPage'));
const SponsorsPage = lazy(() => import('@/pages/sponsors/SponsorsPage'));
const SellersPage = lazy(() => import('@/pages/sellers/SellersPage'));
const SellerHomePage = lazy(() => import('@/pages/seller/SellerHomePage'));
const SellerClientsPage = lazy(() => import('@/pages/seller/SellerClientsPage'));
const SellerSecurityPage = lazy(() => import('@/pages/seller/SellerSecurityPage'));

// 'admin' and 'seller' routes are shown only to that role; 'both' is shared.
export type PanelRoleScope = 'admin' | 'seller' | 'both';

export interface PanelRoute {
  path: string;
  titleKey: string;
  icon: ComponentType;
  component: ComponentType;
  // Page a seller gets at this path, when it differs from component.
  sellerComponent?: ComponentType;
  roles: PanelRoleScope;
}

// The router, the sidebar, the page titles and the command palette all read this list, so a
// page cannot go missing from one of them.
export const PANEL_ROUTES: PanelRoute[] = [
  { path: '/', titleKey: 'menu.dashboard', icon: DashboardOutlined, component: IndexPage, sellerComponent: SellerHomePage, roles: 'both' },
  { path: '/inbounds', titleKey: 'menu.inbounds', icon: ImportOutlined, component: InboundsPage, roles: 'admin' },
  { path: '/clients', titleKey: 'menu.clients', icon: TeamOutlined, component: ClientsPage, sellerComponent: SellerClientsPage, roles: 'both' },
  { path: '/groups', titleKey: 'menu.groups', icon: TagsOutlined, component: GroupsPage, roles: 'admin' },
  { path: '/nodes', titleKey: 'menu.nodes', icon: ClusterOutlined, component: NodesPage, roles: 'admin' },
  { path: '/sellers', titleKey: 'menu.sellers', icon: UserSwitchOutlined, component: SellersPage, roles: 'admin' },
  { path: '/hosts', titleKey: 'menu.hosts', icon: GlobalOutlined, component: HostsPage, roles: 'admin' },
  { path: '/outbound', titleKey: 'menu.outbounds', icon: ExportOutlined, component: XrayPage, roles: 'admin' },
  { path: '/routing', titleKey: 'menu.routing', icon: SwapOutlined, component: XrayPage, roles: 'admin' },
  { path: '/settings', titleKey: 'menu.settings', icon: SettingOutlined, component: SettingsPage, roles: 'admin' },
  { path: '/security', titleKey: 'menu.security', icon: SafetyOutlined, component: SellerSecurityPage, roles: 'seller' },
  { path: '/xray', titleKey: 'menu.xray', icon: ToolOutlined, component: XrayPage, roles: 'admin' },
  { path: '/api-docs', titleKey: 'menu.apiDocs', icon: ApiOutlined, component: ApiDocsPage, roles: 'admin' },
  { path: '/sponsors', titleKey: 'menu.sponsors', icon: CrownOutlined, component: SponsorsPage, roles: 'admin' },
];

export const PANEL_ROUTE_PATHS = PANEL_ROUTES.map((route) => route.path);

export const PANEL_TITLE_KEYS: Record<string, string> = Object.fromEntries(
  PANEL_ROUTES.map((route) => [route.path, route.titleKey]),
);

// The server sets window.X_UI_ROLE on panel pages. It only shapes the UI; the API enforces access.
export function currentPanelRole(): 'admin' | 'seller' {
  return window.X_UI_ROLE === 'seller' ? 'seller' : 'admin';
}

export function panelRouteVisible(route: PanelRoute, role = currentPanelRole()): boolean {
  return route.roles === 'both' || route.roles === role;
}

// Routes a signed-in user may see in menus and the command palette.
export function panelRoutesForCurrentUser(): PanelRoute[] {
  return PANEL_ROUTES.filter((route) => panelRouteVisible(route));
}
