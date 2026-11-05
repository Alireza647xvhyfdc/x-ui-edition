import { useMemo } from 'react';
import { useTranslation } from 'react-i18next';
import { Card, Col, ConfigProvider, Layout, Row, Space, Spin, Statistic, Tag } from 'antd';
import { useQuery } from '@tanstack/react-query';

import { useTheme } from '@/hooks/useTheme';
import { usePageTitle } from '@/hooks/usePageTitle';
import AppSidebar from '@/layouts/AppSidebar';
import { BYTES_PER_GB, expiryLabel, fetchInboundOptions, fetchSellerInfo } from './sellerApi';

export default function SellerHomePage() {
  usePageTitle();
  const { t } = useTranslation();
  const { antdThemeConfig } = useTheme();
  const infoQuery = useQuery({ queryKey: ['seller', 'me'], queryFn: fetchSellerInfo });
  const inboundsQuery = useQuery({ queryKey: ['seller', 'inbounds'], queryFn: fetchInboundOptions });
  const info = infoQuery.data;
  const names = useMemo(
    () => new Map((inboundsQuery.data ?? []).map((ib) => [ib.id, `${ib.remark || ib.protocol} :${ib.port}`])),
    [inboundsQuery.data],
  );

  return (
    <ConfigProvider theme={antdThemeConfig}>
      <Layout>
        <AppSidebar />
        <Layout className="content-shell">
          <Layout.Content id="content-layout" className="content-area">
            <Spin spinning={infoQuery.isLoading} description={t('loading')}>
              {info && (
                <Row gutter={[16, 16]}>
                  <Col span={24}>
                    <Card size="small" title={t('pages.seller.welcome', { name: info.username })}>
                      <Space wrap>
                        {info.blocked ? (
                          <Tag color="red">{t('pages.seller.blocked')}</Tag>
                        ) : (
                          <Tag color="green">{t('pages.seller.active')}</Tag>
                        )}
                        <Tag>{t('pages.seller.expires')}: {expiryLabel(info.expire, t('pages.seller.never'))}</Tag>
                      </Space>
                    </Card>
                  </Col>
                  <Col xs={24} md={8}>
                    <Card size="small">
                      <Statistic
                        title={t('pages.seller.clients')}
                        value={info.clientCount}
                        suffix={info.maxClients ? `/ ${info.maxClients}` : undefined}
                      />
                    </Card>
                  </Col>
                  <Col xs={24} md={8}>
                    <Card size="small">
                      <Statistic
                        title={t('pages.seller.traffic')}
                        value={(info.usedBytes / BYTES_PER_GB).toFixed(2)}
                        suffix={info.totalGB ? `/ ${info.totalGB} GB` : `GB / ${t('pages.seller.unlimited')}`}
                      />
                    </Card>
                  </Col>
                  <Col xs={24} md={8}>
                    <Card size="small" title={t('pages.seller.inbounds')}>
                      {info.inboundIds.length === 0 ? (
                        t('pages.seller.none')
                      ) : (
                        <Space size={[4, 4]} wrap>
                          {info.inboundIds.map((id) => (
                            <Tag key={id}>{names.get(id) ?? `#${id}`}</Tag>
                          ))}
                        </Space>
                      )}
                    </Card>
                  </Col>
                </Row>
              )}
            </Spin>
          </Layout.Content>
        </Layout>
      </Layout>
    </ConfigProvider>
  );
}
