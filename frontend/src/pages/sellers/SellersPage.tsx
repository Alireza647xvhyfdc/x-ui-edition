import { useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Button, Card, ConfigProvider, Form, Input, InputNumber, Layout, Modal, Popconfirm, Select, Space, Table, Tag, message } from 'antd';
import type { TableColumnsType } from 'antd';
import { DeleteOutlined, EditOutlined, PlusOutlined } from '@ant-design/icons';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';

import { useTheme } from '@/hooks/useTheme';
import { usePageTitle } from '@/hooks/usePageTitle';
import { HttpUtil } from '@/utils';
import { setMessageInstance } from '@/utils/messageBus';
import AppSidebar from '@/layouts/AppSidebar';

interface Seller {
  id: number;
  username: string;
  expire: number;
  totalGB: number;
  maxClients: number;
  inboundIds: number[];
  clientCount: number;
  usedBytes: number;
  blocked: boolean;
}

interface InboundOption {
  id: number;
  remark: string;
  port: number;
  protocol: string;
}

interface SellerForm {
  username: string;
  password?: string;
  expireDate?: string;
  totalGB: number;
  maxClients: number;
  inboundIds: number[];
}

const BYTES_PER_GB = 1024 ** 3;

async function fetchSellers(): Promise<Seller[]> {
  const msg = await HttpUtil.get<Seller[]>('/panel/api/sellers/list', undefined, { silent: true });
  if (!msg?.success) throw new Error(msg?.msg || 'Failed to load sellers');
  return msg.obj ?? [];
}

async function fetchInboundOptions(): Promise<InboundOption[]> {
  const msg = await HttpUtil.get<InboundOption[]>('/panel/api/inbounds/list/slim', undefined, { silent: true });
  if (!msg?.success) throw new Error(msg?.msg || 'Failed to load inbounds');
  return msg.obj ?? [];
}

// Expiry is the end of the chosen day in the admin's local time; empty means never.
function expireToDate(expire: number): string | undefined {
  if (!expire) return undefined;
  const d = new Date(expire * 1000);
  const pad = (n: number) => String(n).padStart(2, '0');
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`;
}

function dateToExpire(date?: string): number {
  if (!date) return 0;
  return Math.floor(new Date(`${date}T23:59:59`).getTime() / 1000);
}

export default function SellersPage() {
  usePageTitle();
  const { t } = useTranslation();
  const { antdThemeConfig } = useTheme();
  const [messageApi, messageContextHolder] = message.useMessage();
  useEffect(() => {
    setMessageInstance(messageApi);
  }, [messageApi]);
  const queryClient = useQueryClient();
  const [form] = Form.useForm<SellerForm>();
  const [editing, setEditing] = useState<Seller | null>(null);
  const [modalOpen, setModalOpen] = useState(false);

  const sellersQuery = useQuery({ queryKey: ['sellers'], queryFn: fetchSellers });
  const inboundsQuery = useQuery({ queryKey: ['sellers', 'inbounds'], queryFn: fetchInboundOptions });
  const sellers = useMemo(() => sellersQuery.data ?? [], [sellersQuery.data]);
  const inbounds = useMemo(() => inboundsQuery.data ?? [], [inboundsQuery.data]);
  const inboundLabel = useMemo(
    () => new Map(inbounds.map((ib) => [ib.id, `${ib.remark || ib.protocol} :${ib.port}`])),
    [inbounds],
  );

  const saveMutation = useMutation({
    mutationFn: async (values: SellerForm) => {
      const body = {
        username: values.username,
        password: values.password ?? '',
        expire: dateToExpire(values.expireDate),
        totalGB: values.totalGB ?? 0,
        maxClients: values.maxClients ?? 0,
        inboundIds: values.inboundIds ?? [],
      };
      const url = editing ? `/panel/api/sellers/update/${editing.id}` : '/panel/api/sellers/add';
      const msg = await HttpUtil.post(url, body);
      if (!msg?.success) throw new Error(msg?.msg || t('somethingWentWrong'));
    },
    onSuccess: () => {
      message.success(t('pages.sellers.saved'));
      setModalOpen(false);
      queryClient.invalidateQueries({ queryKey: ['sellers'] });
    },
    onError: (err: Error) => message.error(err.message),
  });

  const deleteMutation = useMutation({
    mutationFn: async (id: number) => {
      const msg = await HttpUtil.post(`/panel/api/sellers/del/${id}`);
      if (!msg?.success) throw new Error(msg?.msg || t('somethingWentWrong'));
    },
    onSuccess: () => {
      message.success(t('pages.sellers.deleted'));
      queryClient.invalidateQueries({ queryKey: ['sellers'] });
    },
    onError: (err: Error) => message.error(err.message),
  });

  const openCreate = () => {
    setEditing(null);
    form.resetFields();
    form.setFieldsValue({ totalGB: 0, maxClients: 0, inboundIds: [] });
    setModalOpen(true);
  };

  const openEdit = (seller: Seller) => {
    setEditing(seller);
    form.setFieldsValue({
      username: seller.username,
      password: '',
      expireDate: expireToDate(seller.expire),
      totalGB: seller.totalGB,
      maxClients: seller.maxClients,
      inboundIds: seller.inboundIds,
    });
    setModalOpen(true);
  };

  const columns: TableColumnsType<Seller> = [
    { title: t('pages.sellers.username'), dataIndex: 'username', key: 'username' },
    {
      title: t('pages.sellers.status'),
      key: 'status',
      render: (_, s) =>
        s.blocked ? <Tag color="red">{t('pages.sellers.blocked')}</Tag> : <Tag color="green">{t('pages.sellers.active')}</Tag>,
    },
    {
      title: t('pages.sellers.expire'),
      key: 'expire',
      render: (_, s) => (s.expire ? expireToDate(s.expire) : t('pages.sellers.expireNever')),
    },
    {
      title: t('pages.sellers.traffic'),
      key: 'traffic',
      render: (_, s) =>
        `${(s.usedBytes / BYTES_PER_GB).toFixed(2)} / ${s.totalGB ? `${s.totalGB} GB` : t('pages.sellers.unlimited')}`,
    },
    {
      title: t('pages.sellers.clients'),
      key: 'clients',
      render: (_, s) => `${s.clientCount} / ${s.maxClients || t('pages.sellers.unlimited')}`,
    },
    {
      title: t('pages.sellers.inbounds'),
      key: 'inbounds',
      render: (_, s) =>
        s.inboundIds.length === 0 ? (
          t('pages.sellers.noInbounds')
        ) : (
          <Space size={[4, 4]} wrap>
            {s.inboundIds.map((id) => (
              <Tag key={id}>{inboundLabel.get(id) ?? `#${id}`}</Tag>
            ))}
          </Space>
        ),
    },
    {
      title: '',
      key: 'actions',
      render: (_, s) => (
        <Space>
          <Button size="small" icon={<EditOutlined />} onClick={() => openEdit(s)} aria-label={t('pages.sellers.edit')} />
          <Popconfirm
            title={t('pages.sellers.deleteConfirm')}
            okText={t('confirm')}
            cancelText={t('cancel')}
            onConfirm={() => deleteMutation.mutate(s.id)}
          >
            <Button size="small" danger icon={<DeleteOutlined />} aria-label={t('pages.sellers.delete')} />
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
              title={t('pages.sellers.title')}
              extra={
                <Button type="primary" icon={<PlusOutlined />} onClick={openCreate}>
                  {t('pages.sellers.add')}
                </Button>
              }
            >
              <Table<Seller>
                rowKey="id"
                size="small"
                loading={sellersQuery.isLoading}
                columns={columns}
                dataSource={sellers}
                scroll={{ x: 900 }}
                locale={{ emptyText: t('pages.sellers.empty') }}
                pagination={false}
              />
            </Card>

            <Modal
              open={modalOpen}
              title={editing ? t('pages.sellers.edit') : t('pages.sellers.add')}
              okText={t('save')}
              cancelText={t('cancel')}
              confirmLoading={saveMutation.isPending}
              onCancel={() => setModalOpen(false)}
              onOk={() => form.validateFields().then((values) => saveMutation.mutate(values))}
              destroyOnHidden
            >
              <Form form={form} layout="vertical">
                <Form.Item name="username" label={t('pages.sellers.username')} rules={[{ required: true }]}>
                  <Input autoComplete="off" />
                </Form.Item>
                <Form.Item
                  name="password"
                  label={t('pages.sellers.password')}
                  extra={editing ? t('pages.sellers.passwordKeep') : undefined}
                  rules={editing ? [] : [{ required: true }]}
                >
                  <Input.Password autoComplete="new-password" />
                </Form.Item>
                <Form.Item name="expireDate" label={t('pages.sellers.expire')} extra={t('pages.sellers.expireHint')}>
                  <Input type="date" />
                </Form.Item>
                <Form.Item name="totalGB" label={t('pages.sellers.totalGB')} extra={t('pages.sellers.zeroUnlimited')}>
                  <InputNumber min={0} style={{ width: '100%' }} />
                </Form.Item>
                <Form.Item name="maxClients" label={t('pages.sellers.maxClients')} extra={t('pages.sellers.zeroUnlimited')}>
                  <InputNumber min={0} precision={0} style={{ width: '100%' }} />
                </Form.Item>
                <Form.Item name="inboundIds" label={t('pages.sellers.inbounds')}>
                  <Select
                    mode="multiple"
                    placeholder={t('pages.sellers.selectInbounds')}
                    options={inbounds.map((ib) => ({ value: ib.id, label: inboundLabel.get(ib.id) ?? `#${ib.id}` }))}
                  />
                </Form.Item>
              </Form>
            </Modal>
          </Layout.Content>
        </Layout>
      </Layout>
    </ConfigProvider>
  );
}
