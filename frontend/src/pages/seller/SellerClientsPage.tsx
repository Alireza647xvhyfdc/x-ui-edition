import { useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import {
  Button,
  Card,
  ConfigProvider,
  Form,
  Input,
  InputNumber,
  Layout,
  Modal,
  Popconfirm,
  Select,
  Space,
  Switch,
  Table,
  Tag,
  message,
} from 'antd';
import type { TableColumnsType } from 'antd';
import { CopyOutlined, DeleteOutlined, EditOutlined, LinkOutlined, PlusOutlined } from '@ant-design/icons';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';

import { useTheme } from '@/hooks/useTheme';
import { usePageTitle } from '@/hooks/usePageTitle';
import { setMessageInstance } from '@/utils/messageBus';
import AppSidebar from '@/layouts/AppSidebar';
import {
  BYTES_PER_GB,
  dayToExpiryMs,
  expiryMsToDay,
  expiryLabel,
  fetchInboundOptions,
  fetchSellerClients,
  fetchSellerInfo,
  getSeller,
  postSeller,
  type SellerClient,
} from './sellerApi';

interface ClientForm {
  email: string;
  limitGB: number;
  expireDay?: string;
  limitIp: number;
  comment?: string;
  inboundIds: number[];
  enable: boolean;
}

const newClientDefaults: Partial<ClientForm> = { limitGB: 0, limitIp: 0, enable: true, inboundIds: [] };

export default function SellerClientsPage() {
  usePageTitle();
  const { t } = useTranslation();
  const { antdThemeConfig } = useTheme();
  const [messageApi, messageContextHolder] = message.useMessage();
  useEffect(() => {
    setMessageInstance(messageApi);
  }, [messageApi]);
  const queryClient = useQueryClient();
  const [form] = Form.useForm<ClientForm>();
  const [editing, setEditing] = useState<SellerClient | null>(null);
  const [modalOpen, setModalOpen] = useState(false);
  const [linksFor, setLinksFor] = useState<{ email: string; text: string } | null>(null);
  const [search, setSearch] = useState('');

  const clientsQuery = useQuery({ queryKey: ['seller', 'clients'], queryFn: fetchSellerClients });
  const infoQuery = useQuery({ queryKey: ['seller', 'me'], queryFn: fetchSellerInfo });
  const inboundsQuery = useQuery({ queryKey: ['seller', 'inbounds'], queryFn: fetchInboundOptions });
  const names = useMemo(
    () => new Map((inboundsQuery.data ?? []).map((ib) => [ib.id, `${ib.remark || ib.protocol} :${ib.port}`])),
    [inboundsQuery.data],
  );
  const rows = useMemo(
    () => (clientsQuery.data ?? []).filter((c) => c.email.toLowerCase().includes(search.toLowerCase())),
    [clientsQuery.data, search],
  );
  const grantedIds = infoQuery.data?.inboundIds ?? [];
  const atLimit = infoQuery.data?.maxClients
    ? (clientsQuery.data?.length ?? 0) >= infoQuery.data.maxClients
    : false;

  const refresh = () => {
    queryClient.invalidateQueries({ queryKey: ['seller'] });
  };

  const saveMutation = useMutation({
    mutationFn: async (values: ClientForm) => {
      const expiryTime = dayToExpiryMs(values.expireDay);
      const totalGB = Math.round((values.limitGB ?? 0) * BYTES_PER_GB);
      if (editing) {
        // The server replaces the whole client row, so send every field the seller keeps.
        await postSeller(`/panel/api/clients/update/${encodeURIComponent(editing.email)}`, {
          email: editing.email,
          id: editing.uuid,
          password: editing.password,
          auth: editing.auth,
          flow: editing.flow,
          security: editing.security,
          subId: editing.subId,
          tgId: editing.tgId,
          enable: values.enable,
          totalGB,
          expiryTime,
          limitIp: values.limitIp ?? 0,
          comment: values.comment ?? '',
        });
      } else {
        await postSeller('/panel/api/clients/add', {
          client: {
            email: values.email,
            enable: true,
            totalGB,
            expiryTime,
            limitIp: values.limitIp ?? 0,
            comment: values.comment ?? '',
          },
          inboundIds: values.inboundIds ?? [],
        });
      }
    },
    onSuccess: () => {
      message.success(t('pages.seller.saved'));
      setModalOpen(false);
      refresh();
    },
    onError: (err: Error) => message.error(err.message),
  });

  const toggleMutation = useMutation({
    mutationFn: async ({ email, enable }: { email: string; enable: boolean }) =>
      postSeller(enable ? '/panel/api/clients/bulkEnable' : '/panel/api/clients/bulkDisable', { emails: [email] }),
    onSuccess: refresh,
    onError: (err: Error) => message.error(err.message),
  });

  const deleteMutation = useMutation({
    mutationFn: async (email: string) => postSeller(`/panel/api/clients/del/${encodeURIComponent(email)}`),
    onSuccess: () => {
      message.success(t('pages.seller.deleted'));
      refresh();
    },
    onError: (err: Error) => message.error(err.message),
  });

  const openCreate = () => {
    setEditing(null);
    form.resetFields();
    form.setFieldsValue(newClientDefaults);
    setModalOpen(true);
  };

  const openEdit = (c: SellerClient) => {
    setEditing(c);
    form.setFieldsValue({
      email: c.email,
      limitGB: c.totalGB / BYTES_PER_GB,
      expireDay: expiryMsToDay(c.expiryTime),
      limitIp: c.limitIp,
      comment: c.comment,
      enable: c.enable,
      inboundIds: c.inboundIds,
    });
    setModalOpen(true);
  };

  const showLinks = async (email: string) => {
    try {
      const links = await getSeller<string[]>(`/panel/api/clients/links/${encodeURIComponent(email)}`);
      setLinksFor({ email, text: (links ?? []).join('\n') });
    } catch (err) {
      message.error((err as Error).message);
    }
  };

  const columns: TableColumnsType<SellerClient> = [
    { title: t('pages.seller.email'), dataIndex: 'email', key: 'email' },
    {
      title: t('pages.seller.status'),
      key: 'status',
      render: (_, c) => (c.enable ? <Tag color="green">{t('pages.seller.active')}</Tag> : <Tag>{t('pages.seller.off')}</Tag>),
    },
    {
      title: t('pages.seller.traffic'),
      key: 'traffic',
      render: (_, c) => {
        const used = (c.traffic?.up ?? 0) + (c.traffic?.down ?? 0);
        return `${(used / BYTES_PER_GB).toFixed(2)} / ${c.totalGB ? `${(c.totalGB / BYTES_PER_GB).toFixed(2)} GB` : t('pages.seller.unlimited')}`;
      },
    },
    {
      title: t('pages.seller.expire'),
      key: 'expire',
      render: (_, c) => expiryLabel(Math.floor(c.expiryTime / 1000), t('pages.seller.never')),
    },
    {
      title: t('pages.seller.inbounds'),
      key: 'inbounds',
      render: (_, c) => (
        <Space size={[4, 4]} wrap>
          {c.inboundIds.map((id) => (
            <Tag key={id}>{names.get(id) ?? `#${id}`}</Tag>
          ))}
        </Space>
      ),
    },
    {
      title: '',
      key: 'actions',
      render: (_, c) => (
        <Space>
          <Button size="small" icon={<LinkOutlined />} onClick={() => showLinks(c.email)} aria-label={t('pages.seller.links')} />
          <Button size="small" icon={<EditOutlined />} onClick={() => openEdit(c)} aria-label={t('pages.seller.edit')} />
          <Switch
            size="small"
            checked={c.enable}
            onChange={(enable) => toggleMutation.mutate({ email: c.email, enable })}
            aria-label={t('pages.seller.enable')}
          />
          <Popconfirm
            title={t('pages.seller.deleteConfirm')}
            okText={t('confirm')}
            cancelText={t('cancel')}
            onConfirm={() => deleteMutation.mutate(c.email)}
          >
            <Button size="small" danger icon={<DeleteOutlined />} aria-label={t('pages.seller.delete')} />
          </Popconfirm>
        </Space>
      ),
    },
  ];

  return (
    <ConfigProvider theme={antdThemeConfig}>
      {messageContextHolder}
      <Layout>
        <AppSidebar />
        <Layout className="content-shell">
          <Layout.Content id="content-layout" className="content-area">
            <Card
              size="small"
              title={t('pages.seller.clientsTitle')}
              extra={
                <Space>
                  <Input.Search
                    allowClear
                    placeholder={t('pages.seller.search')}
                    onChange={(e) => setSearch(e.target.value)}
                    style={{ width: 200 }}
                  />
                  <Button type="primary" icon={<PlusOutlined />} disabled={atLimit} onClick={openCreate}>
                    {t('pages.seller.add')}
                  </Button>
                </Space>
              }
            >
              {atLimit && <p>{t('pages.seller.clientLimitReached')}</p>}
              <Table<SellerClient>
                rowKey="email"
                size="small"
                loading={clientsQuery.isLoading}
                columns={columns}
                dataSource={rows}
                scroll={{ x: 900 }}
                pagination={{ pageSize: 20, hideOnSinglePage: true }}
                locale={{ emptyText: t('pages.seller.empty') }}
              />
            </Card>

            <Modal
              open={modalOpen}
              title={editing ? t('pages.seller.edit') : t('pages.seller.add')}
              okText={t('save')}
              cancelText={t('cancel')}
              confirmLoading={saveMutation.isPending}
              onCancel={() => setModalOpen(false)}
              onOk={() => form.validateFields().then((values) => saveMutation.mutate(values))}
              destroyOnHidden
            >
              <Form form={form} layout="vertical">
                <Form.Item name="email" label={t('pages.seller.email')} rules={[{ required: !editing }]}>
                  <Input disabled={!!editing} autoComplete="off" />
                </Form.Item>
                <Form.Item name="limitGB" label={t('pages.seller.clientLimitGB')} extra={t('pages.seller.zeroUnlimited')}>
                  <InputNumber min={0} style={{ width: '100%' }} />
                </Form.Item>
                <Form.Item name="expireDay" label={t('pages.seller.expire')} extra={t('pages.seller.expireHint')}>
                  <Input type="date" />
                </Form.Item>
                <Form.Item name="limitIp" label={t('pages.seller.limitIp')} extra={t('pages.seller.zeroUnlimited')}>
                  <InputNumber min={0} precision={0} style={{ width: '100%' }} />
                </Form.Item>
                <Form.Item name="comment" label={t('pages.seller.comment')}>
                  <Input autoComplete="off" />
                </Form.Item>
                {editing ? (
                  <Form.Item name="enable" label={t('pages.seller.enable')} valuePropName="checked">
                    <Switch />
                  </Form.Item>
                ) : (
                  <Form.Item name="inboundIds" label={t('pages.seller.inbounds')} rules={[{ required: true }]}>
                    <Select
                      mode="multiple"
                      placeholder={t('pages.seller.selectInbounds')}
                      options={grantedIds.map((id) => ({ value: id, label: names.get(id) ?? `#${id}` }))}
                    />
                  </Form.Item>
                )}
              </Form>
            </Modal>

            <Modal
              open={!!linksFor}
              title={linksFor ? `${t('pages.seller.links')}: ${linksFor.email}` : ''}
              footer={null}
              onCancel={() => setLinksFor(null)}
              destroyOnHidden
            >
              <Input.TextArea readOnly autoSize value={linksFor?.text ?? ''} />
              <Button
                style={{ marginTop: 12 }}
                icon={<CopyOutlined />}
                onClick={() => navigator.clipboard.writeText(linksFor?.text ?? '')}
              >
                {t('pages.seller.copy')}
              </Button>
            </Modal>
          </Layout.Content>
        </Layout>
      </Layout>
    </ConfigProvider>
  );
}
