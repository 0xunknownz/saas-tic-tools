import { useRef, useState } from 'react'
import { useBilling, type BillingPrice } from '@0xunknownz/foundation-tenant-ui'
import { Dialog } from '../components/Dialog'
import { Failure, utcDate } from '../components/Feedback'

const statusLabels = { pending: '待支付', active: '已生效', past_due: '已逾期', canceled: '已取消', expired: '已到期' }
function money(amountCents: number, currency: string) {
  return currency + ' ' + (amountCents / 100).toFixed(2)
}

export function BillingRoute() {
  const billing = useBilling()
  const [confirming, setConfirming] = useState(false)
  const [notice, setNotice] = useState('')
  const lock = useRef(false)
  const subscription = billing.subscription
  const busy = billing.purchasing || billing.canceling
  const canCancel = subscription && ['pending', 'active', 'past_due'].includes(subscription.status) && !subscription.cancelAtPeriodEnd
  const currentPlan = billing.plans.find(plan => plan.code === subscription?.planCode)
  async function purchase(code: string, price: BillingPrice) {
    if (lock.current || billing.loading || billing.provider === 'none') return
    lock.current = true; setNotice('')
    try {
      if (await billing.purchase(code, price.interval, price.currency)) setNotice('购买请求已受理，支付与订阅状态以服务端结果为准。')
    } finally { lock.current = false }
  }
  async function cancel() {
    if (lock.current || !canCancel) return
    lock.current = true; setNotice('')
    try { if (await billing.cancel()) setConfirming(false) }
    finally { lock.current = false }
  }
  return <div className="tic-page">
    <header className="tic-page-heading"><div><h1>账单管理</h1><p>管理当前订阅与套餐。</p></div></header>
    {notice ? <p className="tic-alert" role="status">{notice}</p> : null}
    {billing.loading ? <section className="tic-panel"><p className="tic-empty" role="status">正在加载订阅与套餐…</p></section> : billing.error ? <section className="tic-panel tic-stack"><p>订阅与套餐加载失败，请重试。</p><Failure failure={billing.error}/><button className="tic-button tic-button-secondary" onClick={() => void billing.refresh()}>重新加载账单</button></section> : <>
      {billing.provider === 'none' ? <p className="tic-alert" role="status">未配置支付提供方，暂不可购买套餐；仍可查看当前订阅及使用已有的取消功能。</p> : null}
      <section className="tic-panel tic-stack" aria-labelledby="tic-subscription-heading"><h2 id="tic-subscription-heading">当前订阅</h2>{subscription ? <>
        <div className="tic-subscription"><dl><div><dt>套餐</dt><dd className="tic-plan-name">{currentPlan?.name ?? subscription.planCode}</dd></div><div><dt>订阅状态</dt><dd><span className={'tic-badge' + (subscription.status === 'active' ? ' success' : '')}>{statusLabels[subscription.status] ?? subscription.status}</span>{subscription.cancelAtPeriodEnd && ['active', 'past_due'].includes(subscription.status) ? <p className="tic-hint">到期取消</p> : null}</dd></div><div><dt>金额 / 计费周期</dt><dd>{money(subscription.snapshot.amountCents, subscription.snapshot.currency)}<span className="tic-hint"> / {subscription.snapshot.interval === 'month' ? '月' : '年'}</span></dd></div><div><dt>{subscription.status === 'canceled' || subscription.status === 'expired' ? '原有效期至' : '有效期至'}</dt><dd>{utcDate(subscription.periodEnd)}</dd></div></dl>{canCancel ? <button className="tic-button tic-button-secondary" disabled={busy} onClick={() => setConfirming(true)}>取消订阅</button> : null}</div>
        {subscription.cancelAtPeriodEnd && ['active', 'past_due'].includes(subscription.status) ? <p className="tic-alert" role="status">已安排到期取消，不再续订。{subscription.status === 'active' && subscription.periodEnd ? '当前已付服务可使用至 ' + utcDate(subscription.periodEnd) + '。' : '可用期限以服务端订阅状态为准。'}</p> : subscription.status === 'pending' ? <p className="tic-hint">购买请求待支付，尚未生效；取消将终止该待支付订阅。</p> : subscription.status === 'past_due' ? <p className="tic-hint">订阅已逾期，服务可用性由服务端宽限规则决定；取消后不再续订。</p> : subscription.status === 'canceled' || subscription.status === 'expired' ? <p className="tic-hint">本订阅已结束，不会自动恢复；请选择可用套餐发起新的购买。</p> : <p className="tic-hint">取消将在已付周期结束时生效；实际可用期限以服务端状态为准。</p>}
      </> : <p className="tic-hint">当前没有订阅，可在下方选择套餐。</p>}<Failure failure={billing.cancelError}/></section>
      <section className="tic-panel" aria-labelledby="tic-plans-heading"><div className="tic-panel-heading"><h2 id="tic-plans-heading">可用套餐</h2><span className="tic-hint">按月付或年付购买</span></div><Failure failure={billing.purchaseError}/>{billing.plans.length === 0 ? <p className="tic-empty">暂无可用套餐。</p> : <div className="tic-table-scroll" role="region" aria-label="套餐对比，可横向滚动" tabIndex={0}><table className="tic-table"><thead><tr><th scope="col">套餐</th><th scope="col">功能与额度</th><th scope="col">月付</th><th scope="col">年付</th></tr></thead><tbody>{billing.plans.map(plan => {
        const current = subscription?.planCode === plan.code && ['active', 'past_due'].includes(subscription.status)
        return <tr key={plan.code} className={current ? 'tic-plan-current' : undefined}><td className="tic-plan-info"><strong>{plan.name}</strong>{current ? <p><span className="tic-badge current">当前套餐</span></p> : subscription?.planCode === plan.code && subscription.status === 'pending' ? <p><span className="tic-badge">待支付套餐</span></p> : null}<p className="tic-hint">{plan.description}</p></td><td>{plan.features.length || Object.keys(plan.limits).length ? <ul className="tic-features">{plan.features.map((feature, index) => <li key={index}>{feature}</li>)}{Object.entries(plan.limits).map(([name, amount]) => <li key={name}>{name}：{amount}</li>)}</ul> : <span className="tic-hint">未提供功能与额度</span>}</td>{(['month', 'year'] as const).map(interval => {
          const prices = plan.prices.filter(price => price.interval === interval)
          return <td key={interval}>{prices.length ? <div className="tic-stack">{prices.map(price => <div className="tic-price" key={price.currency}><strong>{money(price.amountCents, price.currency)}<span className="tic-hint"> / {interval === 'month' ? '月' : '年'}</span></strong><button className="tic-button tic-button-secondary" disabled={busy || billing.provider === 'none'} aria-label={'购买' + plan.name + '，' + (interval === 'month' ? '月付' : '年付') + '，' + price.currency} onClick={() => void purchase(plan.code, price)}>{billing.purchasing ? '处理中…' : interval === 'month' ? '按月购买' : '按年购买'}</button></div>)}</div> : <span className="tic-hint">未提供{interval === 'month' ? '月付' : '年付'}价格</span>}</td>
        })}</tr>
      })}</tbody></table></div>}<p className="tic-hint tic-panel-note">仅展示接口提供的价格与额度。购买请求受理后，订阅是否生效以服务端返回为准。</p></section>
    </>}
    {confirming ? <Dialog title="取消订阅" onClose={() => { if (!lock.current) setConfirming(false) }}><div className="tic-stack"><p>{subscription?.status === 'pending' ? '此订阅尚未支付。确认取消后，待支付订阅将终止。' : '取消后不再续订，已付服务保留至当前周期结束。'}{subscription?.periodEnd ? ' 当前有效期至 ' + utcDate(subscription.periodEnd) + '。' : ' 当前有效期未提供，实际生效时间以服务端结果为准。'}</p><Failure failure={billing.cancelError}/><div className="tic-actions"><button className="tic-button tic-button-danger" disabled={busy} onClick={() => void cancel()}>{billing.canceling ? '取消中…' : '确认取消订阅'}</button><button className="tic-button tic-button-secondary" disabled={busy} onClick={() => setConfirming(false)}>保留订阅</button></div></div></Dialog> : null}
  </div>
}
