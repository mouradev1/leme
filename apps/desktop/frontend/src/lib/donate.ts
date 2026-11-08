/**
 * "Buy me a coffee": the same links as the project page. Portuguese opens
 * Mercado Pago (Pix or card, in reais); English opens PayPal.
 */
import { BrowserOpenURL } from '../../wailsjs/runtime/runtime'
import { lang } from './i18n'

const MERCADO_PAGO = 'https://link.mercadopago.com.br/dindinplanejado'
const PAYPAL = 'https://www.paypal.com/donate/?hosted_button_id=XRVF9PMEBWVUA'

export const donate = () => BrowserOpenURL(lang.value === 'pt' ? MERCADO_PAGO : PAYPAL)
