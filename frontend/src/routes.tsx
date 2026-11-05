import { Suspense } from 'react';
import { Navigate, createBrowserRouter, type RouteObject } from 'react-router';
import { Spin } from 'antd';

import PanelLayout from '@/layouts/PanelLayout';
import {
  PANEL_ROUTES,
  currentPanelRole,
  panelRouteVisible,
  type PanelRoute,
} from '@/routes/panelRoutes';

function withSuspense(node: React.ReactNode) {
  return (
    <Suspense
      fallback={
        <div
          style={{
            display: 'flex',
            justifyContent: 'center',
            alignItems: 'center',
            minHeight: '60vh',
          }}
        >
          <Spin size="large" />
        </div>
      }
    >
      {node}
    </Suspense>
  );
}

// A route the signed-in role may not use redirects home, so a seller typing an admin URL
// gets the dashboard instead of a page of refused API calls.
function panelRouteElement(route: PanelRoute) {
  if (!panelRouteVisible(route)) return <Navigate to="/" replace />;
  const Page = currentPanelRole() === 'seller' && route.sellerComponent ? route.sellerComponent : route.component;
  return withSuspense(<Page />);
}

export const routes: RouteObject[] = [
  {
    path: '/',
    element: <PanelLayout />,
    children: PANEL_ROUTES.map((route) => {
      const element = panelRouteElement(route);
      return route.path === '/' ? { index: true, element } : { path: route.path.slice(1), element };
    }),
  },
];

function computeBasename() {
  const raw = (typeof window !== 'undefined' && window.X_UI_BASE_PATH) || '/';
  const trimmed = raw.replace(/\/+$/, '');
  return `${trimmed}/panel`;
}

export const router = createBrowserRouter(routes, {
  basename: computeBasename(),
});
