// Carbon Vue v3 manual plugin for Vite compatibility
// @carbon/vue's UMD entry uses require.context which Vite doesn't support

// UI Shell
import CvHeader from '@carbon/vue/src/components/CvUIShell/CvHeader.vue'
import CvHeaderName from '@carbon/vue/src/components/CvUIShell/CvHeaderName.vue'
import CvHeaderNav from '@carbon/vue/src/components/CvUIShell/CvHeaderNav.vue'
import CvHeaderMenuItem from '@carbon/vue/src/components/CvUIShell/CvHeaderMenuItem.vue'
import CvHeaderGlobalAction from '@carbon/vue/src/components/CvUIShell/CvHeaderGlobalAction.vue'
import CvHeaderMenuButton from '@carbon/vue/src/components/CvUIShell/CvHeaderMenuButton.vue'
import CvSideNav from '@carbon/vue/src/components/CvUIShell/CvSideNav.vue'
import CvSideNavItems from '@carbon/vue/src/components/CvUIShell/CvSideNavItems.vue'
import CvSideNavLink from '@carbon/vue/src/components/CvUIShell/CvSideNavLink.vue'
import CvSideNavMenu from '@carbon/vue/src/components/CvUIShell/CvSideNavMenu.vue'
import CvSideNavMenuItem from '@carbon/vue/src/components/CvUIShell/CvSideNavMenuItem.vue'
import CvContent from '@carbon/vue/src/components/CvUIShell/CvContent.vue'
import CvSkipToContent from '@carbon/vue/src/components/CvUIShell/CvSkipToContent.vue'

// Form & Input
import CvButton from '@carbon/vue/src/components/CvButton/CvButton.vue'
import CvButtonSet from '@carbon/vue/src/components/CvButton/CvButtonSet.vue'
import CvTextInput from '@carbon/vue/src/components/CvTextInput/CvTextInput.vue'
import CvTextArea from '@carbon/vue/src/components/CvTextArea/CvTextArea.vue'
import CvSelect from '@carbon/vue/src/components/CvSelect/CvSelect.vue'
import CvSelectOption from '@carbon/vue/src/components/CvSelect/CvSelectOption.vue'
import CvNumberInput from '@carbon/vue/src/components/CvNumberInput/CvNumberInput.vue'
import CvForm from '@carbon/vue/src/components/CvForm/CvForm.vue'
import CvCheckbox from '@carbon/vue/src/components/CvCheckbox/CvCheckbox.vue'
import CvToggle from '@carbon/vue/src/components/CvToggle/CvToggle.vue'
import CvRadioGroup from '@carbon/vue/src/components/CvRadioButton/CvRadioGroup.vue'
import CvRadioButton from '@carbon/vue/src/components/CvRadioButton/CvRadioButton.vue'

// Data Display
import CvDataTable from '@carbon/vue/src/components/CvDataTable/CvDataTable.vue'
import CvDataTableRow from '@carbon/vue/src/components/CvDataTable/CvDataTableRow.vue'
import CvDataTableCell from '@carbon/vue/src/components/CvDataTable/CvDataTableCell.vue'
import CvDataTableHeading from '@carbon/vue/src/components/CvDataTable/CvDataTableHeading.vue'
import CvTag from '@carbon/vue/src/components/CvTag/CvTag.vue'
import CvList from '@carbon/vue/src/components/CvList/CvList.vue'
import CvListItem from '@carbon/vue/src/components/CvList/CvListItem.vue'
import CvBreadcrumb from '@carbon/vue/src/components/CvBreadcrumb/CvBreadcrumb.vue'
import CvBreadcrumbItem from '@carbon/vue/src/components/CvBreadcrumb/CvBreadcrumbItem.vue'
import CvProgress from '@carbon/vue/src/components/CvProgress/CvProgress.vue'
import CvProgressStep from '@carbon/vue/src/components/CvProgress/CvProgressStep.vue'

// Feedback
import CvModal from '@carbon/vue/src/components/CvModal/CvModal.vue'
import CvLoading from '@carbon/vue/src/components/CvLoading/CvLoading.vue'

// Dropdown
import CvDropdown from '@carbon/vue/src/components/CvDropdown/CvDropdown.vue'
import CvDropdownItem from '@carbon/vue/src/components/CvDropdown/CvDropdownItem.vue'

const components = [
  ['CvHeader', CvHeader],
  ['CvHeaderName', CvHeaderName],
  ['CvHeaderNav', CvHeaderNav],
  ['CvHeaderMenuItem', CvHeaderMenuItem],
  ['CvHeaderGlobalAction', CvHeaderGlobalAction],
  ['CvHeaderMenuButton', CvHeaderMenuButton],
  ['CvSideNav', CvSideNav],
  ['CvSideNavItems', CvSideNavItems],
  ['CvSideNavLink', CvSideNavLink],
  ['CvSideNavMenu', CvSideNavMenu],
  ['CvSideNavMenuItem', CvSideNavMenuItem],
  ['CvContent', CvContent],
  ['CvSkipToContent', CvSkipToContent],
  ['CvButton', CvButton],
  ['CvButtonSet', CvButtonSet],
  ['CvTextInput', CvTextInput],
  ['CvTextArea', CvTextArea],
  ['CvSelect', CvSelect],
  ['CvSelectOption', CvSelectOption],
  ['CvNumberInput', CvNumberInput],
  ['CvForm', CvForm],
  ['CvCheckbox', CvCheckbox],
  ['CvToggle', CvToggle],
  ['CvRadioGroup', CvRadioGroup],
  ['CvRadioButton', CvRadioButton],
  ['CvDataTable', CvDataTable],
  ['CvDataTableRow', CvDataTableRow],
  ['CvDataTableCell', CvDataTableCell],
  ['CvDataTableHeading', CvDataTableHeading],
  ['CvTag', CvTag],
  ['CvList', CvList],
  ['CvListItem', CvListItem],
  ['CvBreadcrumb', CvBreadcrumb],
  ['CvBreadcrumbItem', CvBreadcrumbItem],
  ['CvProgress', CvProgress],
  ['CvProgressStep', CvProgressStep],
  ['CvModal', CvModal],
  ['CvLoading', CvLoading],
  ['CvDropdown', CvDropdown],
  ['CvDropdownItem', CvDropdownItem],
]

export default {
  install(app: any) {
    for (const [name, comp] of components) {
      app.component(name, comp as any)
    }
  },
}
